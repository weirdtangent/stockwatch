package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"graystorm.com/myaws"
)

const marketstack_url = "http://api.marketstack.com/v1/"

func getMarketstackData(aws *session.Session, action string, params map[string]string) (*http.Response, error) {
	httpClient := http.Client{}

	api_access_key, err := myaws.AWSGetSecretKV(aws, "marketstack", "api_access_key")
	if err != nil {
		log.Fatal().Err(err).
			Msg("Failed to get marketstack API key")
	}

	req, err := http.NewRequest("GET", marketstack_url+action, nil)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Failed to construct HTTP request")
	}

	q := req.URL.Query()
	q.Add("access_key", *api_access_key)
	for key, val := range params {
		q.Add(key, val)
	}
	req.URL.RawQuery = q.Encode()

	res, err := httpClient.Do(req)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Failed to perform HTTP request")
	}
	if res.StatusCode != http.StatusOK {
		log.Fatal().
			Stringer("url", req.URL).
			Int("status_code", res.StatusCode).
			Msg("Failed to receive 200 success code from HTTP request")
	}

	return res, nil
}

func updateMarketstackExchanges(aws *session.Session, db *sqlx.DB) (bool, error) {
	var params map[string]string
	res, err := getMarketstackData(aws, "exchanges", params)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Failed to get marketstack data for exchanges")
	}

	defer res.Body.Close()

	var apiResponse MSExchangeResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	for _, MSExchangeData := range apiResponse.Data {
		if MSExchangeData.Acronym != "" {
			// grab the country_id we'll need, create new record if needed
			var country = &Country{0, MSExchangeData.Country_code, MSExchangeData.Country, "", ""}
			country, err := createOrUpdateCountry(db, country)
			if err != nil {
				log.Fatal().Err(err).
					Str("country_code", MSExchangeData.Country_code).
					Msg("Failed to create/update country for exchange")
			}

			// use marketstack data to create or update exchange
			var exchange = &Exchange{0, MSExchangeData.Acronym, MSExchangeData.Name, country.Country_id, MSExchangeData.City, "", ""}
			_, err = createOrUpdateExchange(db, exchange)
			if err != nil {
				log.Fatal().Err(err).
					Str("acronym", MSExchangeData.Acronym).
					Msg("Failed to create/update exchange")
			}
		}
	}
	return true, nil
}

func updateMarketstackTicker(aws *session.Session, db *sqlx.DB, symbol string) (*Ticker, error) {
	var params map[string]string
	res, err := getMarketstackData(aws, fmt.Sprintf("tickers/%s/eod", symbol), params)
	if err != nil {
		log.Fatal().Err(err).
			Str("symbol", symbol).
			Msg("Failed to get marketstack data for ticker")
	}

	defer res.Body.Close()

	var apiResponse MSEndOfDayResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	var MSEndOfDayData MSEndOfDayData = apiResponse.Data

	// grab the exchange's country_id we'll need, create new record if needed
	var country = &Country{0, MSEndOfDayData.StockExchange.Country_code, MSEndOfDayData.StockExchange.Country, "", ""}
	country, err = getOrCreateCountry(db, country)
	if err != nil {
		log.Fatal().Err(err).
			Str("country_code", MSEndOfDayData.StockExchange.Country_code).
			Msg("Failed to create/update country for exchange")
	}

	// grab the exchange_id we'll need, create new record if needed
	var exchange = &Exchange{0, MSEndOfDayData.StockExchange.Acronym, MSEndOfDayData.StockExchange.Name, country.Country_id, MSEndOfDayData.StockExchange.City, "", ""}
	exchange, err = getOrCreateExchange(db, exchange)
	if err != nil {
		log.Fatal().Err(err).
			Str("acronym", MSEndOfDayData.StockExchange.Acronym).
			Msg("Failed to create/update exchange")
	}

	// use marketstack data to create or update ticker
	var ticker = &Ticker{0, MSEndOfDayData.Symbol, exchange.Exchange_id, MSEndOfDayData.Name, "", ""}
	ticker, err = createOrUpdateTicker(db, ticker)
	if err != nil {
		log.Fatal().Err(err).
			Str("symbol", MSEndOfDayData.Symbol).
			Msg("Failed to create/update ticker")
	}

	// finally, lets roll through all the EOD price data we got and make sure we have it all
	for _, MSIndexData := range apiResponse.Data.EndOfDay {
		// use marketstack data to create or update dailies
		var daily = &Daily{0, ticker.Ticker_id, MSIndexData.Price_date, MSIndexData.Open_price, MSIndexData.High_price, MSIndexData.Low_price, MSIndexData.Close_price, MSIndexData.Volume, "", ""}
		if daily.Volume > 0 {
			_, err = createOrUpdateDaily(db, daily)
			if err != nil {
				log.Fatal().Err(err).
					Str("symbol", ticker.Ticker_symbol).
					Int64("ticker_id", ticker.Ticker_id).
					Msg("Failed to create/update EOD for ticker")
			}
		}
	}

	return ticker, nil
}

func searchMarketstackTicker(aws *session.Session, db *sqlx.DB, search string) (*Ticker, error) {
	params := make(map[string]string)
	params["search"] = search

	res, err := getMarketstackData(aws, "tickers", params)
	if err != nil {
		log.Fatal().Err(err).
			Str("search", search).
			Msg("Failed to get marketstack data for a search")
	}

	defer res.Body.Close()

	var apiResponse MSTickerResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	var firstResult Ticker
	for _, MSTickerData := range apiResponse.Data {
		if MSTickerData.Symbol != "" {
			// grab the exchange's country_id we'll need, create new record if needed
			var country = &Country{0, MSTickerData.StockExchange.Country_code, MSTickerData.StockExchange.Country, "", ""}
			country, err = getOrCreateCountry(db, country)
			if err != nil {
				log.Fatal().Err(err).
					Str("country_code", MSTickerData.StockExchange.Country_code).
					Msg("Failed to create/update country for exchange")
			}

			// grab the exchange_id we'll need, create new record if needed
			var exchange = &Exchange{0, MSTickerData.StockExchange.Acronym, MSTickerData.StockExchange.Name, country.Country_id, MSTickerData.StockExchange.City, "", ""}
			exchange, err = getOrCreateExchange(db, exchange)
			if err != nil {
				log.Fatal().Err(err).
					Str("acronym", MSTickerData.StockExchange.Acronym).
					Msg("Failed to create/update exchange")
			}

			// use marketstack data to create or update ticker
			var ticker = &Ticker{0, MSTickerData.Symbol, exchange.Exchange_id, MSTickerData.Name, "", ""}
			ticker, err = createOrUpdateTicker(db, ticker)
			if err != nil {
				log.Fatal().Err(err).
					Str("symbol", MSTickerData.Symbol).
					Msg("Failed to create/update ticker")
			}
			if firstResult.Ticker_symbol == "" {
				firstResult = *ticker
			}
		}
	}

	return updateMarketstackTicker(aws, db, firstResult.Ticker_symbol)
}
