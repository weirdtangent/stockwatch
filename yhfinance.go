package main

import (
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
func fetchTickerInfoFromYH(deps *Dependencies, symbol string) (Ticker, error) {
	redisPool := deps.redisPool
	secrets := deps.secrets
	sublog := deps.logger.With().Str("symbol", symbol).Logger()

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
		response, err := yhfinance.GetYHFinanceStockSummary(&sublog, apiKey, apiHost, symbol)
		if err != nil {
			return Ticker{}, err
		}
		_, err = redisConn.Do("SET", redisKey, response, "EX", 60*60*24)
		if err != nil {
			sublog.Error().Err(err).Str("ticker", symbol).Str("redis_key", redisKey).Msg("failed to save to redis")
		}
	}

	var summaryResponse yhfinance.YHStockSummaryResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&summaryResponse)

	// can't create exchange - all we get is ExchangeCode and I can't find a
	// table to translate those to Exchange MIC or Acronym... so I have to
	// link them manually for now
	exchange := Exchange{ExchangeCode: summaryResponse.Price.ExchangeCode}
	err = exchange.getByCode(deps)
	if err != nil {
		sublog.Error().Err(err).Str("ticker", summaryResponse.QuoteType.Symbol).Str("exchange_code", summaryResponse.Price.ExchangeCode).Msg("failed to find exchange_code matched to exchange mic record")
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
		"",
		sql.NullTime{},
		"",
		time.Now(),
		time.Now(),
	}
	err = ticker.createOrUpdate(deps)
	if err != nil {
		sublog.Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to create or update ticker")
		return Ticker{}, err
	}

	tickerDescription := TickerDescription{0, ticker.TickerId, summaryResponse.SummaryProfile.LongBusinessSummary, time.Now(), time.Now()}
	err = tickerDescription.createOrUpdate(deps)
	if err != nil {
		sublog.Error().Err(err).Str("ticker", ticker.TickerSymbol).Msg("failed to create ticker description")
	}

	// create upgrade/downgrade recommendations
	for _, updown := range summaryResponse.UpgradeDowngradeHistory.Histories {
		updownDate := time.Unix(updown.GradeDate, 0)
		UpDown := TickerUpDown{0, ticker.TickerId, updown.Action, updown.FromGrade, updown.ToGrade, sql.NullTime{Valid: true, Time: updownDate}, updown.Firm, "", time.Now(), time.Now()}
		UpDown.createIfNew(deps)
	}

	// create/update ticker_attributes
	ticker.createOrUpdateAttribute(deps, "sector", "", summaryResponse.SummaryProfile.Sector)
	ticker.createOrUpdateAttribute(deps, "industry", "", summaryResponse.SummaryProfile.Industry)
	ticker.createOrUpdateAttribute(deps, "short_ratio", "", summaryResponse.DefaultKeyStatistics.ShortRatio.Fmt)
	ticker.createOrUpdateAttribute(deps, "last_split_date", "", summaryResponse.DefaultKeyStatistics.LastSplitDate.Fmt)
	ticker.createOrUpdateAttribute(deps, "last_dividend_date", "", summaryResponse.DefaultKeyStatistics.LastDividendDate.Fmt)
	ticker.createOrUpdateAttribute(deps, "shares_short", "", summaryResponse.DefaultKeyStatistics.SharesShort.Fmt)
	ticker.createOrUpdateAttribute(deps, "float_shares", "", summaryResponse.DefaultKeyStatistics.FloatShares.Fmt)
	ticker.createOrUpdateAttribute(deps, "forward_eps", "", summaryResponse.DefaultKeyStatistics.ForwardEPS.Fmt)
	ticker.createOrUpdateAttribute(deps, "enterprize_to_revenue", "", summaryResponse.DefaultKeyStatistics.EnterprizeToRevenue.Fmt)
	ticker.createOrUpdateAttribute(deps, "enterprize_to_ebita", "", summaryResponse.DefaultKeyStatistics.EnterprizeToEbita.Fmt)

	return ticker, nil
}

// load ticker up-to-date quote
func fetchTickerQuoteFromYH(deps *Dependencies, symbol string) (yhfinance.YHQuote, error) {
	redisPool := deps.redisPool
	secrets := deps.secrets
	sublog := deps.logger

	sublog.With().Str("symbol", symbol)

	redisConn := redisPool.Get()
	defer redisConn.Close()

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]

	var quote yhfinance.YHQuote

	// pull recent response from redis (20 sec expire), or go get from YF
	redisKey := "yhfinance/quote/" + symbol
	response, err := redis.String(redisConn.Do("GET", redisKey))
	if err == nil && response != "" && !skipRedisChecks {
		sublog.Info().Str("redis_key", redisKey).Msg("redis cache hit")
	} else {
		var err error
		quoteParams := map[string]string{"symbols": symbol}
		response, err = yhfinance.GetFromYHFinance(sublog, apiKey, apiHost, "marketQuote", quoteParams)
		if err != nil {
			log.Warn().Err(err).Msg("failed to retrieve quote")
			return quote, err
		}
		if response != "" {
			_, err = redisConn.Do("SET", redisKey, response, "EX", 20)
			if err != nil {
				sublog.Error().Err(err).Str("redis_key", redisKey).Msg("failed to save to redis")
			}
		}
	}

	var quoteResponse yhfinance.YHGetQuotesResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&quoteResponse)

	if len(quoteResponse.QuoteResponse.Quotes) == 0 {
		sublog.Warn().Msg(fmt.Sprintf("%#v", strings.NewReader(response)))
		sublog.Warn().Msg("failed to get quote response back from yhfinance")
		return quote, nil
	}
	quote = quoteResponse.QuoteResponse.Quotes[0]

	return quote, nil
}

func loadMultiTickerQuotes(deps *Dependencies, symbols []string) (map[string]yhfinance.YHQuote, error) {
	redisPool := deps.redisPool
	secrets := deps.secrets
	sublog := deps.logger

	redisConn := redisPool.Get()
	defer redisConn.Close()

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]

	quotes := map[string]yhfinance.YHQuote{}

	var err error
	quoteParams := map[string]string{"symbols": strings.Join(symbols, ",")}
	sublog.Info().Str("symbols", strings.Join(symbols, ",")).Msg("getting multi-symbol quote from yhfinance")
	fullResponse, err := yhfinance.GetFromYHFinance(sublog, apiKey, apiHost, "marketQuote", quoteParams)
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
func loadTickerEODsFromYH(deps *Dependencies, ticker Ticker) error {
	secrets := deps.secrets
	sublog := deps.logger

	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	currentDateStr := currentDate.Format("2006-01-02")
	currentTimeStr := currentDate.Format("15:04:05")

	historicalParams := map[string]string{"symbol": ticker.TickerSymbol}

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]
	if apiKey == "" || apiHost == "" {
		sublog.Fatal().Msg("apiKey or apiHost secret is missing")
	}
	response, err := yhfinance.GetFromYHFinance(sublog, apiKey, apiHost, "stockHistorical", historicalParams)
	if err != nil {
		sublog.Warn().Err(err).Str("ticker", ticker.TickerSymbol).Msg("failed to retrieve historical prices")
		return err
	}

	var historicalResponse yhfinance.YHHistoricalDataResponse
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
		priceDatetime, _ := time.Parse(sqlDatetimeParseType, priceDate+" "+priceTime)
		tickerDaily := TickerDaily{0, ticker.TickerId, priceDate, priceTime, priceDatetime, price.Open, price.High, price.Low, price.Close, price.Volume, time.Now(), time.Now()}
		err = tickerDaily.createOrUpdate(deps)
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		log.Warn().Err(err).Str("ticker", ticker.TickerSymbol).Msg("failed to load at least one historical price")
	}

	for _, split := range historicalResponse.Events {
		tickerSplit := TickerSplit{0, ticker.TickerId, time.Unix(split.Date, 0), split.SplitRatio, time.Now(), time.Now()}
		err = tickerSplit.createIfNew(deps)
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
func jumpSearch(deps *Dependencies, searchString string) (SearchResultTicker, error) {
	var searchResult SearchResultTicker

	searchResults, err := listSearch(deps, searchString, "ticker")
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
func listSearch(deps *Dependencies, searchString string, resultTypes string) ([]SearchResult, error) {
	secrets := deps.secrets
	sublog := deps.logger

	apiKey := secrets["yhfinance_rapidapi_key"]
	apiHost := secrets["yhfinance_rapidapi_host"]

	searchResults := make([]SearchResult, 100)

	searchParams := map[string]string{"q": searchString, "region": "US"}
	response, err := yhfinance.GetFromYHFinance(sublog, apiKey, apiHost, "autocomplete", searchParams)
	if err != nil {
		return searchResults, err
	}

	searchCount := 0
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
			searchCount++
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
			if quoteResult.Type == "Option" {
				continue
			}
			exchange := Exchange{ExchangeCode: quoteResult.ExchangeCode}
			err := exchange.getByCode(deps)
			if err != nil {
				sublog.Error().Err(err).Str("exchange_code", quoteResult.ExchangeCode).Msg("failed to get exchange by code")
				continue
			}
			if exchange.ExchangeId > 0 {
				searchCount++
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

	sublog.Info().Str("search_string", searchString).Int("results_count", searchCount).Msg("Search returned results")

	return searchResults, nil
}
