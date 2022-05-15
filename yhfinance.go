package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/yhfinance"
)

// fetch ticker info (and possibly new exchange) from yhfinance
func fetchTickerInfoFromYH(deps *Dependencies, sublog zerolog.Logger, symbol string) (Ticker, error) {
	redisPool := deps.redisPool
	secrets := deps.secrets
	sublog = sublog.With().Str("symbol", symbol).Logger()

	redisConn := redisPool.Get()
	defer redisConn.Close()

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]

	// pull recent response from redis (1 day expire), or go get from YF
	redisKey := "yhfinance/summary/" + symbol
	response, err := redis.String(redisConn.Do("GET", redisKey))
	if err == nil && !skipRedisChecks {
		sublog.Info().Str("redis_key", redisKey).Msg("redis cache hit")
	} else {
		start := time.Now()
		response, err = yhfinance.GetYHFinanceStockSummary(&sublog, apiKey, apiHost, symbol)
		sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Msg("timer: yhfinance stockSummary")
		if err != nil {
			return Ticker{}, err
		}

		_, err = redisConn.Do("SET", redisKey, response, "EX", 60*60*24)
		if err != nil {
			sublog.Error().Err(err).Str("ticker", symbol).Str("redis_key", redisKey).Msg("failed to save to redis")
		}
	}

	var summaryResponse yhfinance.YHStockSummaryResponse
	err = json.NewDecoder(strings.NewReader(response)).Decode(&summaryResponse)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to decode json")
	}

	// can't create exchange - all we get is ExchangeCode and I can't find a
	// table to translate those to Exchange MIC or Acronym... so I have to
	// link them manually for now
	exchange := Exchange{ExchangeCode: summaryResponse.Price.ExchangeCode}
	err = exchange.getByCode(deps, sublog)
	if err != nil {
		sublog.Error().Err(err).Str("ticker", summaryResponse.QuoteType.Symbol).Str("exchange_code", summaryResponse.Price.ExchangeCode).Msg("failed to find exchange_code matched to exchange mic record")
		return Ticker{}, err
	}

	// create/update ticker
	ticker := Ticker{
		0,
		"",
		summaryResponse.QuoteType.Symbol,
		summaryResponse.Price.QuoteType,
		summaryResponse.QuoteType.Market,
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
		summaryResponse.Price.RegularMarketPrice.Raw,
		summaryResponse.Price.RegularMarketPreviousClose.Raw,
		summaryResponse.Price.RegularMarketVolume.Raw,
		time.Unix(summaryResponse.Price.RegularMarketTime, 0),
		"",
		time.Now(),
		"",
		time.Now(),
		time.Now(),
	}
	err = ticker.createOrUpdate(deps, sublog)
	if err != nil {
		sublog.Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to create or update ticker")
		return Ticker{}, err
	}
	if ticker.TickerSymbol == "" || ticker.TickerId == 0 {
		sublog.Fatal().Interface("ticker", ticker).Str("quotetype_symbol", summaryResponse.QuoteType.Symbol).Msg("ticker object is not saved")
	}

	tickerDescription := TickerDescription{0, "", ticker.TickerId, summaryResponse.SummaryProfile.LongBusinessSummary, time.Now(), time.Now()}
	err = tickerDescription.createOrUpdate(deps, sublog)
	if err != nil {
		sublog.Error().Err(err).Str("ticker", ticker.TickerSymbol).Msg("failed to create ticker description")
	}

	// create upgrade/downgrade recommendations
	for _, updown := range summaryResponse.UpgradeDowngradeHistory.Histories {
		updownDate := time.Unix(updown.GradeDate, 0)
		UpDown := TickerUpDown{0, "", ticker.TickerId, updown.Action, updown.FromGrade, updown.ToGrade, sql.NullTime{Valid: true, Time: updownDate}, updown.Firm, "", time.Now(), time.Now()}
		UpDown.createIfNew(deps, sublog)
	}

	// create/update ticker_attributes
	ticker.createOrUpdateAttribute(deps, sublog, "sector", "", summaryResponse.SummaryProfile.Sector)
	ticker.createOrUpdateAttribute(deps, sublog, "industry", "", summaryResponse.SummaryProfile.Industry)
	ticker.createOrUpdateAttribute(deps, sublog, "short_ratio", "", summaryResponse.DefaultKeyStatistics.ShortRatio.Fmt)
	ticker.createOrUpdateAttribute(deps, sublog, "last_split_date", "", summaryResponse.DefaultKeyStatistics.LastSplitDate.Fmt)
	ticker.createOrUpdateAttribute(deps, sublog, "last_dividend_date", "", summaryResponse.DefaultKeyStatistics.LastDividendDate.Fmt)
	ticker.createOrUpdateAttribute(deps, sublog, "shares_short", "", summaryResponse.DefaultKeyStatistics.SharesShort.Fmt)
	ticker.createOrUpdateAttribute(deps, sublog, "float_shares", "", summaryResponse.DefaultKeyStatistics.FloatShares.Fmt)
	ticker.createOrUpdateAttribute(deps, sublog, "forward_eps", "", summaryResponse.DefaultKeyStatistics.ForwardEPS.Fmt)
	ticker.createOrUpdateAttribute(deps, sublog, "enterprize_to_revenue", "", summaryResponse.DefaultKeyStatistics.EnterprizeToRevenue.Fmt)
	ticker.createOrUpdateAttribute(deps, sublog, "enterprize_to_ebita", "", summaryResponse.DefaultKeyStatistics.EnterprizeToEbita.Fmt)

	return ticker, nil
}

// load ticker up-to-date quote
func fetchTickerQuoteFromYH(deps *Dependencies, sublog zerolog.Logger, ticker Ticker) (yhfinance.YHQuote, error) {
	secrets := deps.secrets

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]

	quote := yhfinance.YHQuote{}
	quoteParams := map[string]string{"symbols": ticker.TickerSymbol, "region": ticker.TickerMarket}
	response, err := yhfinance.GetFromYHFinance(&sublog, apiKey, apiHost, "marketQuote", quoteParams)
	if err != nil {
		return quote, err
	}

	quoteResponse := yhfinance.YHGetQuotesResponse{}
	err = json.NewDecoder(strings.NewReader(response)).Decode(&quoteResponse)
	if err != nil {
		return quote, err
	}
	if len(quoteResponse.QuoteResponse.Quotes) == 0 {
		return quote, fmt.Errorf("failed to get response data back from yhfinance")
	}

	quote = quoteResponse.QuoteResponse.Quotes[0]

	return quote, nil
}

func loadMultiTickerQuotes(deps *Dependencies, sublog zerolog.Logger, symbols []string) (map[string]yhfinance.YHQuote, error) {
	redisPool := deps.redisPool
	secrets := deps.secrets

	redisConn := redisPool.Get()
	defer redisConn.Close()

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]

	quotes := map[string]yhfinance.YHQuote{}

	start := time.Now()
	var err error
	quoteParams := map[string]string{"symbols": strings.Join(symbols, ",")}
	sublog.Info().Str("symbols", strings.Join(symbols, ",")).Msg("getting multi-symbol quote from yhfinance")
	fullResponse, err := yhfinance.GetFromYHFinance(&sublog, apiKey, apiHost, "marketQuote", quoteParams)
	sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Msg("timer: yhfinance multi marketQuote")
	if err != nil {
		log.Warn().Err(err).Str("symbols", strings.Join(symbols, ",")).Msg("failed to retrieve quote")
		return quotes, err
	}

	var quoteResponse yhfinance.YHGetQuotesResponse
	json.NewDecoder(strings.NewReader(fullResponse)).Decode(&quoteResponse)

	for n, response := range quoteResponse.QuoteResponse.Quotes {
		symbol := quoteResponse.QuoteResponse.Quotes[n].Symbol
		redisKey := "yhfinance/quote/" + symbol
		_, err = redisConn.Do("SET", redisKey, response, "EX", 20)
		if err != nil {
			sublog.Error().Err(err).Str("ticker", symbol).Str("redis_key", redisKey).Msg("failed to save to redis")
		}

		sublog.Info().Str("symbol", symbol).Msg("found yhfinance quote response for {symbol}")
		quotes[symbol] = quoteResponse.QuoteResponse.Quotes[n]
	}

	return quotes, nil
}

// load ticker historical prices
func fetchTickerEODsFromYH(deps *Dependencies, sublog zerolog.Logger, ticker Ticker) error {
	secrets := deps.secrets

	historicalParams := map[string]string{"symbol": ticker.TickerSymbol}

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]
	if apiKey == "" || apiHost == "" {
		sublog.Fatal().Msg("apiKey or apiHost secret is missing")
	}
	response, err := yhfinance.GetFromYHFinance(&sublog, apiKey, apiHost, "stockHistorical", historicalParams)
	if err != nil {
		sublog.Warn().Err(err).Str("ticker", ticker.TickerSymbol).Msg("failed to retrieve historical prices")
		return err
	}

	var historicalResponse yhfinance.YHHistoricalDataResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&historicalResponse)

	var lastErr error
	for _, price := range historicalResponse.Prices {
		tickerDaily := TickerDaily{0, "", ticker.TickerId, time.Unix(price.Date, 0), price.Open, price.High, price.Low, price.Close, price.Volume, time.Now(), time.Now()}
		err = tickerDaily.createOrUpdate(deps, sublog)
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		log.Warn().Err(err).Str("ticker", ticker.TickerSymbol).Msg("failed to load at least one historical price")
	}

	for _, split := range historicalResponse.Events {
		tickerSplit := TickerSplit{0, "", ticker.TickerId, time.Unix(split.Date, 0), split.SplitRatio, time.Now(), time.Now()}
		err = tickerSplit.createIfNew(deps, sublog)
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		log.Warn().Err(err).Str("ticker", ticker.TickerSymbol).Msg("failed to load at least one historical split")
	}

	return nil
}

// search for ticker and return highest scored quote symbol
func jumpSearch(deps *Dependencies, sublog zerolog.Logger, searchString string) (SearchResultTicker, error) {
	var searchResult SearchResultTicker

	searchResults, err := listSearch(deps, sublog, searchString, "ticker")
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
	if searchResult.TickerSymbol == "" {
		return searchResult, fmt.Errorf("sorry, the search returned zero results")
	}

	return searchResult, nil
}

// search for ticker or news
func listSearch(deps *Dependencies, sublog zerolog.Logger, searchString string, resultTypes string) ([]SearchResult, error) {
	secrets := deps.secrets

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]

	start := time.Now()
	searchResults := make([]SearchResult, 100)
	searchParams := map[string]string{"q": searchString, "region": "US"}
	response, err := yhfinance.GetFromYHFinance(&sublog, apiKey, apiHost, "autocomplete", searchParams)
	sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Msg("timer: yhfinance autocomplete")
	if err != nil {
		return searchResults, err
	}

	newsCount := 0
	tickerCount := 0
	var searchResponse yhfinance.YHAutoCompleteResponse
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
			newsCount++
			searchResults = append(searchResults, SearchResult{
				ResultType: "news",
				News: SearchResultNews{
					Publisher:   newsResult.Publisher,
					Title:       newsResult.Title,
					Type:        newsResult.Type,
					URL:         newsResult.URL,
					PublishDate: time.Unix(newsResult.PublishTime, 0).Format("Jan 2 15:04 MST 2006"),
				},
				Ticker: SearchResultTicker{},
			})
		}
	}

	if resultTypes == "ticker" || resultTypes == "both" {
		for _, quoteResult := range searchResponse.Quotes {
			if quoteResult.Type == "Option" {
				continue
			}
			exchange := Exchange{ExchangeCode: quoteResult.ExchangeCode}
			err := exchange.getByCode(deps, sublog)
			if err != nil {
				sublog.Error().Err(err).Str("symbol", quoteResult.Symbol).Str("exchange_code", quoteResult.ExchangeCode).Msg("skipping {symbol} with unknown {exchange_code}")
				continue
			}
			if exchange.ExchangeId > 0 {
				tickerCount++
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

	sublog.Info().Str("search_string", searchString).Int("news_count", newsCount).Int("ticker_count", tickerCount).Msg("Search results")

	return searchResults, nil
}
