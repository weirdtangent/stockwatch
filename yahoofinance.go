package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/yahoofinance"
)

// load new ticker (and possibly new exchange)
func findTicker(ctx context.Context, symbol string) (*Ticker, error) {
	logger := log.Ctx(ctx)
	var ticker Ticker
	var exchange Exchange

	apiKey := ctx.Value("yahoofinance_apikey").(string)
	apiHost := ctx.Value("yahoofinance_apihost").(string)

	summaryParams := map[string]string{"symbol": symbol}
	response, err := yahoofinance.GetFromYahooFinance(&apiKey, &apiHost, "summary", summaryParams)
	if err != nil {
		return &ticker, err
	}

	var summaryResponse yahoofinance.YFSummaryResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&summaryResponse)

	exchange = Exchange{0, summaryResponse.QuoteType.Acronym, summaryResponse.Price.ExchangeName, 0, "", "", ""}
	err = exchange.getOrCreate(ctx)
	if err != nil {
		logger.Error().Err(err).
			Str("ticker", summaryResponse.QuoteType.Symbol).
			Str("exchange", summaryResponse.QuoteType.Acronym).
			Msg("Failed to create or update exchange")
		return &ticker, err
	}

	ticker = Ticker{0, summaryResponse.QuoteType.Symbol, exchange.ExchangeId, summaryResponse.QuoteType.ShortName, "", ""}
	err = ticker.createOrUpdate(ctx)
	if err != nil {
		logger.Error().Err(err).
			Str("ticker", summaryResponse.QuoteType.Symbol).
			Str("exchange", summaryResponse.QuoteType.Acronym).
			Msg("Failed to create or update ticker")
		return &ticker, err
	}
	return &ticker, nil
}

// load ticker historical prices
func loadTickerPrices(ctx context.Context, ticker *Ticker) error {
	apiKey := ctx.Value("yahoofinance_apikey").(string)
	apiHost := ctx.Value("yahoofinance_apihost").(string)
	logger := log.Ctx(ctx)

	historicalParams := map[string]string{"symbol": ticker.TickerSymbol}
	response, err := yahoofinance.GetFromYahooFinance(&apiKey, &apiHost, "historical_data", historicalParams)
	if err != nil {
		logger.Warn().Err(err).
			Str("ticker", ticker.TickerSymbol).
			Msg("Failed to find historical prices")
		return err
	}

	var historicalResponse yahoofinance.YFHistoricalDataResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&historicalResponse)

	var lastErr error
	for _, price := range historicalResponse.Prices {
		priceDate := RealDateForDB(price.Date)
		tickerDaily := TickerDaily{0, ticker.TickerId, priceDate, price.Open, price.High, price.Low, price.Close, price.Volume, "", ""}
		err = tickerDaily.createIfNew(ctx)
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
		splitDate := RealDateForDB(split.Date)
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

	return lastErr
}

// search for ticker and return highest scored quote symbol
func jumpSearch(ctx context.Context, searchString string) (SearchResult, error) {
	var searchResult SearchResult

	searchResults, err := listSearch(ctx, searchString, "ticker")
	if err != nil {
		return searchResult, err
	}
	if len(searchResults) == 0 {
		return searchResult, fmt.Errorf("Sorry, the search returned zero results")
	}

	var highestScore int64 = 0
	for _, result := range searchResults {
		if result.Ticker.SearchScore > highestScore {
			searchResult = result
			highestScore = searchResult.Ticker.SearchScore
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
		for _, result := range searchResponse.News {
			searchResults = append(searchResults, SearchResult{
				ResultType: "news",
				News: SearchResultNews{
					Publisher:   result.Publisher,
					Title:       result.Title,
					Type:        result.Type,
					URL:         result.URL,
					PublishDate: RealDateForHuman(result.PublishTime),
				},
				Ticker: SearchResultTicker{},
			})
		}
	}

	if resultTypes == "ticker" || resultTypes == "both" {
		for _, result := range searchResponse.Quotes {
			searchResults = append(searchResults, SearchResult{
				ResultType: "ticker",
				News:       SearchResultNews{},
				Ticker: SearchResultTicker{
					TickerSymbol:    result.Symbol,
					ExchangeAcronym: result.Acronym,
					Type:            result.Type,
					ShortName:       result.ShortName,
					LongName:        result.LongName,
					SearchScore:     result.Score,
				},
			})
		}
	}

	return searchResults, nil
}
