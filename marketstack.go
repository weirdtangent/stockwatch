package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/marketstack"
)

func fetchExchanges(ctx context.Context) (int, error) {
	db := ctx.Value("db").(*sqlx.DB)
	api_access_key := ctx.Value("marketstack_key").(string)

	exchanges, err := marketstack.FetchExchanges(api_access_key)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, MSExchangeData := range exchanges {
		switch MSExchangeData.Acronym {
		case "":
			break
		default:
			// grab the countryId we'll need, create new record if needed
			var country = &Country{0, MSExchangeData.CountryCode, MSExchangeData.CountryName, "", ""}
			country, err := createOrUpdateCountry(db, country)
			if err != nil {
				log.Error().Err(err).
					Str("country_code", MSExchangeData.CountryCode).
					Msg("Failed to create/update country for exchange")
			}

			// use marketstack data to create or update exchange
			var exchange = &Exchange{0, MSExchangeData.Acronym, MSExchangeData.Mic, MSExchangeData.Name, country.CountryId, MSExchangeData.City, "", ""}
			_, err = createOrUpdateExchange(db, exchange)
			if err != nil {
				log.Error().Err(err).
					Str("acronym", MSExchangeData.Acronym).
					Msg("Failed to create/update exchange")
			}

			log.Info().Str("acronym", exchange.ExchangeAcronym).Msg("Exchange created/updated")
			count += 1
		}
	}
	log.Info().Int("count", count).Msg("Exchanges updated from marketstack")
	return count, nil
}

func fetchMarketIndexes(ctx context.Context) (int, error) {
	db := ctx.Value("db").(*sqlx.DB)
	api_access_key := ctx.Value("marketstack_key").(string)

	marketindexes, err := marketstack.FetchMarketIndexes(api_access_key)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, MSMarketIndexData := range marketindexes {
		switch MSMarketIndexData.Symbol {
		case "":
			break
		default:
			// grab the countryId we'll need, can't create new record if needed because we don't have CountryCode
			country, err := getCountryByName(db, MSMarketIndexData.CountryName)
			if err != nil {
				log.Error().Err(err).
					Str("country_name", MSMarketIndexData.CountryName).
					Msg("Failed to find country for marketindex")
			}

			// grab the currencyId we'll need, create new record if needed
			var currency = &Currency{0, MSMarketIndexData.Currency.CurrencyCode, MSMarketIndexData.Currency.CurrencyName, MSMarketIndexData.Currency.CurrencySymbol, "", "", ""}
			currency, err = createOrUpdateCurrency(db, currency)
			if err != nil {
				log.Error().Err(err).
					Str("currency_code", MSMarketIndexData.Currency.CurrencyCode).
					Msg("Failed to create/update currency for index")
			}

			// use marketstack data to create or update index
			var marketindex = &MarketIndex{0, MSMarketIndexData.Symbol, "INDX", MSMarketIndexData.Name, country.CountryId, MSMarketIndexData.HasIntraday, MSMarketIndexData.HasEOD, currency.CurrencyId, "", ""}
			_, err = createOrUpdateMarketIndex(db, marketindex)
			if err != nil {
				log.Error().Err(err).
					Str("symbol", MSMarketIndexData.Symbol).
					Msg("Failed to create/update index")
			}

			log.Info().Str("marketindex", marketindex.MarketIndexSymbol).Msg("MarketIndex created/updated")
			count += 1
		}
	}
	log.Info().Int("count", count).Msg("Indexes updated from marketstack")
	return count, nil
}

func fetchCurrencies(ctx context.Context) (int, error) {
	db := ctx.Value("db").(*sqlx.DB)
	api_access_key := ctx.Value("marketstack_key").(string)

	currencies, err := marketstack.FetchCurrencies(api_access_key)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, MSCurrencyData := range currencies {
		var currency = &Currency{0, MSCurrencyData.CurrencyCode, MSCurrencyData.CurrencyName, MSCurrencyData.CurrencySymbol, MSCurrencyData.CurrencySymbolNative, "", ""}
		currency, err = createOrUpdateCurrency(db, currency)
		if err != nil {
			log.Error().Err(err).
				Str("currency_code", MSCurrencyData.CurrencyCode).
				Msg("Failed to create/update currency for index")
		}

		log.Info().Str("currency_code", currency.CurrencyCode).Msg("Currency created/updated")
		count += 1
	}
	log.Info().Int("count", count).Msg("Currencies updated from marketstack")
	return count, nil
}

func fetchTicker(ctx context.Context, symbol string, exchangeMic string) (*Ticker, error) {
	db := ctx.Value("db").(*sqlx.DB)
	api_access_key := ctx.Value("marketstack_key").(string)
	awssess := ctx.Value("awssess").(*session.Session)
	messages := ctx.Value("messages").(*[]Message)

	EOD, err := marketstack.FetchTickerEOD(api_access_key, symbol, exchangeMic, 31, 0)
	if err != nil {
		return nil, err
	}

	// grab the exchange's countryId we'll need, create new record if needed
	var country = &Country{0, EOD.StockExchange.CountryCode, EOD.StockExchange.CountryName, "", ""}
	country, err = getOrCreateCountry(db, country)
	if err != nil {
		log.Error().Err(err).
			Str("country_code", EOD.StockExchange.CountryCode).
			Msg("Failed to create/update country for exchange")
		return nil, err
	}

	// grab the exchange_id we'll need, create new record if needed
	var exchange = &Exchange{0, EOD.StockExchange.Acronym, EOD.StockExchange.Mic, EOD.StockExchange.Name, country.CountryId, EOD.StockExchange.City, "", ""}
	exchange, err = getOrCreateExchange(db, exchange)
	if err != nil {
		log.Error().Err(err).
			Str("acronym", EOD.StockExchange.Acronym).
			Msg("Failed to create/update exchange")
		return nil, err
	}

	// use marketstack data to create or update ticker
	var ticker = &Ticker{0, EOD.Symbol, exchange.ExchangeId, EOD.Name, "", ""}
	ticker, err = createOrUpdateTicker(db, ticker)
	if err != nil {
		log.Error().Err(err).
			Str("symbol", EOD.Symbol).
			Str("acronym", EOD.StockExchange.Acronym).
			Msg("Failed to create/update ticker")
		return nil, err
	}

	// finally, lets roll through all the EOD price data we got and make sure we have it all
	var anyErr error
	for _, MSIndexData := range EOD.EndOfDay {
		// use marketstack data to create or update dailies
		var ticker_daily = &TickerDaily{0, ticker.TickerId, MSIndexData.PriceDate, MSIndexData.OpenPrice, MSIndexData.HighPrice, MSIndexData.LowPrice, MSIndexData.ClosePrice, MSIndexData.Volume, "", ""}
		if ticker_daily.Volume > 0 {
			_, err = createOrUpdateTickerDaily(db, ticker_daily)
			if err != nil {
				anyErr = err
			}
		} else {
			log.Warn().
				Str("symbol", ticker.TickerSymbol).
				Int64("ticker_id", ticker.TickerId).
				Str("price_date", MSIndexData.PriceDate).
				Msg("Failed to get any volume for today")
		}
	}
	if anyErr != nil {
		log.Error().Err(anyErr).
			Str("symbol", ticker.TickerSymbol).
			Int64("ticker_id", ticker.TickerId).
			Msg("Failed to create/update 1 or more EOD for ticker")
	}

	scheduled := ticker.ScheduleEODUpdate(awssess, db)
	if scheduled {
		*messages = append(*messages, Message{fmt.Sprintf("Scheduled task to load historical EOD prices for %s", ticker.TickerSymbol), "success"})
	}

	return ticker, nil
}

func fetchTickerIntraday(ctx context.Context, ticker Ticker, exchange *Exchange, intradate string) error {
	db := ctx.Value("db").(*sqlx.DB)
	api_access_key := ctx.Value("marketstack_key").(string)

	intradays, err := marketstack.FetchTickerIntradays(api_access_key, ticker.TickerSymbol, exchange.ExchangeMic, intradate)
	if err != nil {
		return err
	}

	// country isn't provided, exchange is but to do an intraday
	// we have to already have that info, so we'll skip doing
	// any updates for that from this API call

	var anyErr error
	var priorVol float32
	for _, MSIntradayData := range intradays {
		timeSlot := MSIntradayData.PriceDate[11:16]
		if timeSlot >= "14:30" && timeSlot < "21:05" {
			// use marketstack data to create or update intradays
			var tickerintraday = &TickerIntraday{0, ticker.TickerId, MSIntradayData.PriceDate, MSIntradayData.LastPrice, MSIntradayData.Volume - priorVol, "", ""}
			_, err = createOrUpdateTickerIntraday(db, tickerintraday)
			if err != nil {
				anyErr = err
			}
			priorVol = MSIntradayData.Volume
		}
	}
	if anyErr != nil {
		log.Fatal().Err(anyErr).
			Str("symbol", ticker.TickerSymbol).
			Int64("tickerId", ticker.TickerId).
			Msg("Failed to create/update 1 or more Intraday for ticker")
	}

	return nil
}

func fetchMarketIndexIntraday(ctx context.Context, marketindex MarketIndex, intradate string) error {
	db := ctx.Value("db").(*sqlx.DB)
	api_access_key := ctx.Value("marketstack_key").(string)

	intradays, err := marketstack.FetchMarketIndexIntradays(api_access_key, marketindex.MarketIndexSymbol, intradate)
	if err != nil {
		return err
	}

	// country isn't provided, exchange is but to do an intraday
	// we have to already have that info, so we'll skip doing
	// any updates for that from this API call

	// lets roll through all the intraday price data we got and make sure we have it all
	var anyErr error
	var priorVol float32
	for _, MSIntradayData := range intradays {
		timeSlot := MSIntradayData.PriceDate[11:16]
		if timeSlot >= "14:30" && timeSlot < "21:05" {
			// use marketstack data to create or update intradays
			var intraday = &MarketIndexIntraday{0, marketindex.MarketIndexId, MSIntradayData.PriceDate, MSIntradayData.LastPrice, MSIntradayData.Volume - priorVol, "", ""}
			_, err = createOrUpdateMarketIndexIntraday(db, intraday)
			if err != nil {
				anyErr = err
			}
			priorVol = MSIntradayData.Volume
		}
	}
	if anyErr != nil {
		log.Fatal().Err(anyErr).
			Str("symbol", marketindex.MarketIndexSymbol).
			Int64("marketindex_id", marketindex.MarketIndexId).
			Msg("Failed to create/update 1 or more Intraday for market index")
	}

	return nil
}

func jumpsearchMarketstackTicker(ctx context.Context, searchString string) (SearchResult, error) {
	api_access_key := ctx.Value("marketstack_key").(string)

	result, err := marketstack.JumpsearchMarketstackTicker(api_access_key, searchString)

	searchResult := SearchResult{result.Symbol, result.StockExchange.Acronym, result.StockExchange.CountryName, result.Name}
	return searchResult, err
}

func listsearchMarketstackTicker(ctx context.Context, searchString string) ([]SearchResult, error) {
	api_access_key := ctx.Value("marketstack_key").(string)

	searchResults := make([]SearchResult, 0)

	results, err := marketstack.ListsearchMarketstackTicker(api_access_key, searchString)
	for _, result := range results {
		searchResults = append(searchResults, SearchResult{result.Symbol, result.StockExchange.Acronym, result.StockExchange.CountryName, result.Name})
	}

	return searchResults, err
}
