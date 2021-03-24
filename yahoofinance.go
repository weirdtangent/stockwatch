package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/yahoofinance"
)

// load new ticker (and possibly new exchange)
func loadTicker(ctx context.Context, symbol string) (*Ticker, error) {
	logger := log.Ctx(ctx)
	redisPool := ctx.Value("redisPool").(*redis.Pool)

	redisConn := redisPool.Get()
	defer redisConn.Close()

	var ticker *Ticker

	apiKey := ctx.Value("yahoofinance_apikey").(string)
	apiHost := ctx.Value("yahoofinance_apihost").(string)

	// pull recent response from redis (1 day expire), or go get from YF
	redisKey := "yahoofinance/summary/" + symbol
	response, err := redis.String(redisConn.Do("GET", redisKey))
	if err == nil {
		log.Info().Str("redis_key", redisKey).Msg("redis cache hit")
	} else {
		var err error
		summaryParams := map[string]string{"symbol": symbol}
		response, err = yahoofinance.GetFromYahooFinance(&apiKey, &apiHost, "summary", summaryParams)
		if err != nil {
			logger.Warn().Err(err).
				Str("ticker", symbol).
				Msg("Failed to retrieve ticker")
			return ticker, err
		}
		_, err = redisConn.Do("SET", redisKey, response, "EX", 60*60*24)
		if err != nil {
			logger.Error().Err(err).
				Str("ticker", symbol).
				Str("redis_key", redisKey).
				Msg("Failed to save to redis")
		}
	}

	var summaryResponse yahoofinance.YFSummaryResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&summaryResponse)

	// can't create exchange - all we get is ExchangeCode and I can't find a
	// table to translate those to Exchange MIC or Acronym... so I have to
	// link them manually for now
	exchangeId, err := getExchangeByCode(ctx, summaryResponse.Price.ExchangeCode)
	if err != nil {
		logger.Error().Err(err).
			Str("ticker", summaryResponse.QuoteType.Symbol).
			Str("exchange_code", summaryResponse.Price.ExchangeCode).
			Msg("Failed to find exchange_code matched to exchange mic record")
		return ticker, err
	}

	// create/update ticker
	ticker = &Ticker{
		0, // id
		summaryResponse.QuoteType.Symbol,
		exchangeId,
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
		"now()",
		"", // ms_performance_id
		"", // create_datetime
		"", // update_datetime
	}
	err = ticker.createOrUpdate(ctx)
	if err != nil {
		logger.Error().Err(err).
			Str("ticker", summaryResponse.QuoteType.Symbol).
			Str("exchange_code", summaryResponse.QuoteType.ExchangeCode).
			Msg("Failed to create or update ticker")
		return ticker, err
	}

	tickerDescription := &TickerDescription{0, ticker.TickerId, summaryResponse.SummaryProfile.LongBusinessSummary, "", ""}
	err = tickerDescription.createIfNew(ctx)
	if err != nil {
		logger.Error().Err(err).
			Str("ticker", ticker.TickerSymbol).
			Msg("Failed to create ticker description")
	}

	// create upgrade/downgrade recommendations
	for _, updown := range summaryResponse.UpgradeDowngradeHistory.Histories {
		updownDate := UnixToDatetimeStr(updown.GradeDate)
		UpDown := TickerUpDown{0, ticker.TickerId, updown.Action, updown.FromGrade, updown.ToGrade, updownDate, updown.Firm, "", "", ""}
		UpDown.createIfNew(ctx)
	}

	// create/update ticker_attributes
	ticker.createOrUpdateAttribute(ctx, "sector", summaryResponse.SummaryProfile.Sector)
	ticker.createOrUpdateAttribute(ctx, "industry", summaryResponse.SummaryProfile.Industry)
	ticker.createOrUpdateAttribute(ctx, "short_ratio", summaryResponse.DefaultKeyStatistics.ShortRatio.Fmt)
	ticker.createOrUpdateAttribute(ctx, "last_split_date", summaryResponse.DefaultKeyStatistics.LastSplitDate.Fmt)
	ticker.createOrUpdateAttribute(ctx, "last_dividend_date", summaryResponse.DefaultKeyStatistics.LastDividendDate.Fmt)
	ticker.createOrUpdateAttribute(ctx, "shares_short", summaryResponse.DefaultKeyStatistics.SharesShort.Fmt)
	ticker.createOrUpdateAttribute(ctx, "float_shares", summaryResponse.DefaultKeyStatistics.FloatShares.Fmt)
	ticker.createOrUpdateAttribute(ctx, "shares_outstanding", summaryResponse.DefaultKeyStatistics.SharesOutstanding.Fmt)

	return ticker, nil
}

// load ticker up-to-date quote
func loadTickerQuote(ctx context.Context, symbol string) (yahoofinance.YFQuote, error) {
	logger := log.Ctx(ctx)
	redisPool := ctx.Value("redisPool").(*redis.Pool)

	redisConn := redisPool.Get()
	defer redisConn.Close()

	apiKey := ctx.Value("yahoofinance_apikey").(string)
	apiHost := ctx.Value("yahoofinance_apihost").(string)

	var quote yahoofinance.YFQuote

	// pull recent response from redis (20 sec expire), or go get from YF
	redisKey := "yahoofinance/quote/" + symbol
	response, err := redis.String(redisConn.Do("GET", redisKey))
	if err == nil {
		log.Info().Str("redis_key", redisKey).Msg("redis cache hit")
	} else {
		var err error
		quoteParams := map[string]string{"symbols": symbol}
		response, err = yahoofinance.GetFromYahooFinance(&apiKey, &apiHost, "quote", quoteParams)
		if err != nil {
			logger.Warn().Err(err).
				Str("ticker", symbol).
				Msg("Failed to retrieve quote")
			return quote, err
		}
		_, err = redisConn.Do("SET", redisKey, response, "EX", 20)
		if err != nil {
			logger.Error().Err(err).
				Str("ticker", symbol).
				Str("redis_key", redisKey).
				Msg("Failed to save to redis")
		}
	}

	var quoteResponse yahoofinance.YFGetQuotesResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&quoteResponse)

	quote = quoteResponse.QuoteResponse.Quotes[0]

	return quote, nil
}

// load ticker historical prices
func loadTickerEODs(ctx context.Context, ticker *Ticker) error {
	apiKey := ctx.Value("yahoofinance_apikey").(string)
	apiHost := ctx.Value("yahoofinance_apihost").(string)
	logger := log.Ctx(ctx)

	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	currentDateStr := currentDate.Format("2006-01-02")
	currentTimeStr := currentDate.Format("15:04:05")

	historicalParams := map[string]string{"symbol": ticker.TickerSymbol}
	response, err := yahoofinance.GetFromYahooFinance(&apiKey, &apiHost, "historical", historicalParams)
	if err != nil {
		logger.Warn().Err(err).
			Str("ticker", ticker.TickerSymbol).
			Msg("Failed to retrieve historical prices")
		return err
	}

	var historicalResponse yahoofinance.YFHistoricalDataResponse
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
		tickerDaily := TickerDaily{0, ticker.TickerId, priceDate, priceTime, price.Open, price.High, price.Low, price.Close, price.Volume, "", ""}
		err = tickerDaily.createOrUpdate(ctx)
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		logger.Warn().Err(err).
			Str("ticker", ticker.TickerSymbol).
			Msg("Failed to load at least one historical price")
	}

	for _, split := range historicalResponse.Events {
		splitDate := FormatUnixTime(split.Date, "2006-01-02")
		tickerSplit := TickerSplit{0, ticker.TickerId, splitDate, split.SplitRatio, "", ""}
		err = tickerSplit.createIfNew(ctx)
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		logger.Warn().Err(err).
			Str("ticker", ticker.TickerSymbol).
			Msg("Failed to load at least one historical split")
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
		return searchResult, fmt.Errorf("Sorry, the search returned zero results")
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
	apiKey := ctx.Value("yahoofinance_apikey").(string)
	apiHost := ctx.Value("yahoofinance_apihost").(string)

	searchResults := make([]SearchResult, 0)

	searchParams := map[string]string{"q": searchString}
	response, err := yahoofinance.GetFromYahooFinance(&apiKey, &apiHost, "autocomplete", searchParams)
	if err != nil {
		return searchResults, err
	}

	var searchResponse yahoofinance.YFAutoCompleteResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&searchResponse)

	if len(searchResponse.Quotes) == 0 && len(searchResponse.News) == 0 {
		return searchResults, fmt.Errorf("Sorry, the search returned zero results")
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
			exchangeId, err := getExchangeByCode(ctx, quoteResult.ExchangeCode)
			if err == nil && exchangeId > 0 {
				exchange, _ := getExchangeById(ctx, exchangeId)
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

	log.Info().
		Str("search_string", searchString).
		Int("results_count", len(searchResults)).
		Msg("Search returned results")

	return searchResults, nil
}
