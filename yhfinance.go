package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/yhfinance"
)

// fetch ticker info (and possibly new exchange) from yhfinance
func fetchTickerInfo(ctx context.Context, symbol string) (Ticker, error) {
	redisPool := ctx.Value(ContextKey("redisPool")).(*redis.Pool)

	redisConn := redisPool.Get()
	defer redisConn.Close()

	apiKey := ctx.Value(ContextKey("yhfinance_apikey")).(string)
	apiHost := ctx.Value(ContextKey("yhfinance_apihost")).(string)

	// pull recent response from redis (1 day expire), or go get from YF
	redisKey := "yhfinance/summary/" + symbol
	response, err := redis.String(redisConn.Do("GET", redisKey))
	if err == nil && !skipRedisChecks {
		log.Info().Str("redis_key", redisKey).Msg("redis cache hit")
	} else {
		var err error
		summaryParams := map[string]string{"symbol": symbol}
		log.Info().Str("symbol", symbol).Msg("get {symbol} info from yhfinance api")
		response, err = yhfinance.GetFromYHFinance(ctx, &apiKey, &apiHost, "summary", summaryParams)
		if err != nil {
			log.Warn().Err(err).Str("ticker", symbol).Msg("failed to retrieve ticker")
			return Ticker{}, err
		}
		_, err = redisConn.Do("SET", redisKey, response, "EX", 60*60*24)
		if err != nil {
			log.Error().Err(err).Str("ticker", symbol).Str("redis_key", redisKey).Msg("failed to save to redis")
		}
	}

	var summaryResponse yhfinance.YFSummaryResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&summaryResponse)

	// can't create exchange - all we get is ExchangeCode and I can't find a
	// table to translate those to Exchange MIC or Acronym... so I have to
	// link them manually for now
	exchange := Exchange{ExchangeCode: summaryResponse.Price.ExchangeCode}
	err = exchange.getByCode(ctx)
	if err != nil {
		log.Error().Err(err).Str("ticker", summaryResponse.QuoteType.Symbol).Str("exchange_code", summaryResponse.Price.ExchangeCode).Msg("failed to find exchange_code matched to exchange mic record")
		return Ticker{}, err
	}

	// create/update ticker
	ticker := Ticker{
		0,
		summaryResponse.QuoteType.Symbol,
		exchange.ExchangeId,
		summaryResponse.QuoteType.ShortName,
		summaryResponse.QuoteType.LongName,
		summaryResponse.SummaryProfile.Address1,
		summaryResponse.SummaryProfile.City,
		summaryResponse.SummaryProfile.State,
		summaryResponse.SummaryProfile.Zip,
		summaryResponse.SummaryProfile.Country,
		summaryResponse.SummaryProfile.Website,
		summaryResponse.SummaryProfile.Phone,
		summaryResponse.SummaryProfile.Sector,
		summaryResponse.SummaryProfile.Industry,
		sql.NullTime{},
		"",
		sql.NullTime{},
		sql.NullTime{},
	}
	err = ticker.createOrUpdate(ctx)
	if err != nil {
		log.Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to create or update ticker")
		return Ticker{}, err
	}

	tickerDescription := TickerDescription{0, ticker.TickerId, summaryResponse.SummaryProfile.LongBusinessSummary, sql.NullTime{}, sql.NullTime{}}
	err = tickerDescription.createOrUpdate(ctx)
	if err != nil {
		log.Error().Err(err).Str("ticker", ticker.TickerSymbol).Msg("Failed to create ticker description")
	}

	// create upgrade/downgrade recommendations
	for _, updown := range summaryResponse.UpgradeDowngradeHistory.Histories {
		updownDate := UnixToDatetimeStr(updown.GradeDate)
		UpDown := TickerUpDown{0, ticker.TickerId, updown.Action, updown.FromGrade, updown.ToGrade, updownDate, updown.Firm, "", sql.NullTime{}, sql.NullTime{}}
		UpDown.createIfNew(ctx)
	}

	// create/update ticker_attributes
	ticker.createOrUpdateAttribute(ctx, "sector", "", summaryResponse.SummaryProfile.Sector)
	ticker.createOrUpdateAttribute(ctx, "industry", "", summaryResponse.SummaryProfile.Industry)
	ticker.createOrUpdateAttribute(ctx, "short_ratio", "", summaryResponse.DefaultKeyStatistics.ShortRatio.Fmt)
	ticker.createOrUpdateAttribute(ctx, "last_split_date", "", summaryResponse.DefaultKeyStatistics.LastSplitDate.Fmt)
	ticker.createOrUpdateAttribute(ctx, "last_dividend_date", "", summaryResponse.DefaultKeyStatistics.LastDividendDate.Fmt)
	ticker.createOrUpdateAttribute(ctx, "shares_short", "", summaryResponse.DefaultKeyStatistics.SharesShort.Fmt)
	ticker.createOrUpdateAttribute(ctx, "float_shares", "", summaryResponse.DefaultKeyStatistics.FloatShares.Fmt)
	ticker.createOrUpdateAttribute(ctx, "forward_eps", "", summaryResponse.DefaultKeyStatistics.ForwardEPS.Fmt)
	ticker.createOrUpdateAttribute(ctx, "enterprize_to_revenue", "", summaryResponse.DefaultKeyStatistics.EnterprizeToRevenue.Fmt)
	ticker.createOrUpdateAttribute(ctx, "enterprize_to_ebita", "", summaryResponse.DefaultKeyStatistics.EnterprizeToEbita.Fmt)

	return ticker, nil
}

// load ticker up-to-date quote
func loadTickerQuote(ctx context.Context, symbol string) (yhfinance.YFQuote, error) {
	redisPool := ctx.Value(ContextKey("redisPool")).(*redis.Pool)

	redisConn := redisPool.Get()
	defer redisConn.Close()

	apiKey := ctx.Value(ContextKey("yhfinance_apikey")).(string)
	apiHost := ctx.Value(ContextKey("yhfinance_apihost")).(string)

	var quote yhfinance.YFQuote

	// pull recent response from redis (20 sec expire), or go get from YF
	redisKey := "yhfinance/quote/" + symbol
	response, err := redis.String(redisConn.Do("GET", redisKey))
	if err == nil && !skipRedisChecks {
		log.Info().Str("redis_key", redisKey).Msg("redis cache hit")
	} else {
		var err error
		quoteParams := map[string]string{"symbols": symbol}
		response, err = yhfinance.GetFromYHFinance(ctx, &apiKey, &apiHost, "quote", quoteParams)
		if err != nil {
			log.Warn().Err(err).Str("ticker", symbol).Msg("Failed to retrieve quote")
			return quote, err
		}
		_, err = redisConn.Do("SET", redisKey, response, "EX", 20)
		if err != nil {
			log.Error().Err(err).Str("ticker", symbol).Str("redis_key", redisKey).Msg("Failed to save to redis")
		}
	}

	var quoteResponse yhfinance.YFGetQuotesResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&quoteResponse)

	quote = quoteResponse.QuoteResponse.Quotes[0]

	return quote, nil
}

// load ticker historical prices
func loadTickerEODs(ctx context.Context, ticker Ticker) error {
	apiKey := ctx.Value(ContextKey("yhfinance_apikey")).(string)
	apiHost := ctx.Value(ContextKey("yhfinance_apihost")).(string)

	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	currentDateStr := currentDate.Format("2006-01-02")
	currentTimeStr := currentDate.Format("15:04:05")

	historicalParams := map[string]string{"symbol": ticker.TickerSymbol}
	response, err := yhfinance.GetFromYHFinance(ctx, &apiKey, &apiHost, "historical", historicalParams)
	if err != nil {
		log.Warn().Err(err).Str("ticker", ticker.TickerSymbol).Msg("Failed to retrieve historical prices")
		return err
	}

	var historicalResponse yhfinance.YFHistoricalDataResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&historicalResponse)

	var lastErr error
	for _, price := range historicalResponse.Prices {
		priceDate := FormatUnixTime(price.Date, "2006-01-02")
		// if the price is for TODAY I will use the current time
		// instead of the timestamp
		priceTime := FormatUnixTime(price.Date, "15:04:05")
		if priceDate == currentDateStr {
			priceTime = currentTimeStr
		}
		tickerDaily := TickerDaily{0, ticker.TickerId, priceDate, priceTime, price.Open, price.High, price.Low, price.Close, price.Volume, sql.NullTime{}, sql.NullTime{}}
		err = tickerDaily.createOrUpdate(ctx)
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		log.Warn().Err(err).Str("ticker", ticker.TickerSymbol).Msg("Failed to load at least one historical price")
	}

	for _, split := range historicalResponse.Events {
		splitDate := FormatUnixTime(split.Date, "2006-01-02")
		tickerSplit := TickerSplit{0, ticker.TickerId, splitDate, split.SplitRatio, sql.NullTime{}, sql.NullTime{}}
		err = tickerSplit.createIfNew(ctx)
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		log.Warn().Err(err).Str("ticker", ticker.TickerSymbol).Msg("Failed to load at least one historical split")
	}

	return nil
}

// search for ticker and return highest scored quote symbol
func jumpSearch(ctx context.Context, searchString string) (SearchResultTicker, error) {
	var searchResult SearchResultTicker

	searchResults, err := listSearch(ctx, searchString, "ticker")
	if err != nil {
		return searchResult, err
	}
	if len(searchResults) == 0 {
		return searchResult, fmt.Errorf("sorry, the search returned zero results")
	}

	var highestScore float64 = 0
	for _, result := range searchResults {
		if result.ResultType == "ticker" && result.Ticker.SearchScore > highestScore {
			searchResult = result.Ticker
			highestScore = result.Ticker.SearchScore
		}
	}

	return searchResult, nil
}

// search for ticker or news
func listSearch(ctx context.Context, searchString string, resultTypes string) ([]SearchResult, error) {
	apiKey := ctx.Value(ContextKey("yhfinance_apikey")).(string)
	apiHost := ctx.Value(ContextKey("yhfinance_apihost")).(string)

	searchResults := make([]SearchResult, 100)

	searchParams := map[string]string{"q": searchString}
	response, err := yhfinance.GetFromYHFinance(ctx, &apiKey, &apiHost, "autocomplete", searchParams)
	if err != nil {
		return searchResults, err
	}

	var searchResponse yhfinance.YFAutoCompleteResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&searchResponse)

	if resultTypes == "ticker" && len(searchResponse.Quotes) == 0 {
		return searchResults, fmt.Errorf("sorry, the search returned zero results")
	}
	if resultTypes == "news" && len(searchResponse.News) == 0 {
		return searchResults, fmt.Errorf("sorry, the search returned zero results")
	}
	if resultTypes == "both" && len(searchResponse.Quotes)+len(searchResponse.News) == 0 {
		return searchResults, fmt.Errorf("sorry, the search returned zero results")
	}

	if resultTypes == "news" || resultTypes == "both" {
		for _, newsResult := range searchResponse.News {
			searchResults = append(searchResults, SearchResult{
				ResultType: "news",
				News: SearchResultNews{
					Publisher:   newsResult.Publisher,
					Title:       newsResult.Title,
					Type:        newsResult.Type,
					URL:         newsResult.URL,
					PublishDate: FormatUnixTime(newsResult.PublishTime, ""),
				},
				Ticker: SearchResultTicker{},
			})
		}
	}

	if resultTypes == "ticker" || resultTypes == "both" {
		for _, quoteResult := range searchResponse.Quotes {
			exchange := Exchange{ExchangeCode: quoteResult.ExchangeCode}
			err := exchange.getByCode(ctx)
			if err == nil && exchange.ExchangeId > 0 {
				searchResults = append(searchResults, SearchResult{
					ResultType: "ticker",
					News:       SearchResultNews{},
					Ticker: SearchResultTicker{
						TickerSymbol: quoteResult.Symbol,
						ExchangeMic:  exchange.ExchangeMic,
						Type:         quoteResult.Type,
						ShortName:    quoteResult.ShortName,
						LongName:     quoteResult.LongName,
						SearchScore:  quoteResult.Score,
					},
				})
			}
		}
	}

	log.Info().Str("search_string", searchString).Int("results_count", len(searchResults)).Msg("Search returned results")

	return searchResults, nil
}
