package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/myaws"
)

const marketstack_url = "https://api.marketstack.com/v1/"

type MSExchangeData struct {
	Name        string `json:"name"`
	Acronym     string `json:"acronym"`
	Mic         string `json:"mic"`
	CountryName string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
}

type MSCurrencyData struct {
	CurrencyName         string `json:"name"`
	CurrencyCode         string `json:"code"`
	CurrencySymbol       string `json:"symbol"`
	CurrencySymbolNative string `json:"symbol_native"`
}

type MSIndexCurrencyData struct {
	CurrencyName   string `json:"name"`
	CurrencyCode   string `json:"code"`
	CurrencySymbol string `json:"symbol"`
}

type MSMarketIndexData struct {
	Symbol      string              `json:"symbol"`
	Name        string              `json:"name"`
	CountryName string              `json:"country"`
	Currency    MSIndexCurrencyData `json:"currency"`
	HasIntraday bool                `json:"has_intraday"`
	HasEOD      bool                `json:"has_eod"`
}

type MSMarketIndexesData struct {
	MarketIndexes []MSMarketIndexData `json:"indexes"`
}

type MSIndexData struct {
	Symbol     string  `json:"symbol"`
	Exchange   string  `json:"exchange"`
	PriceDate  string  `json:"date"`
	OpenPrice  float32 `json:"open"`
	HighPrice  float32 `json:"high"`
	LowPrice   float32 `json:"low"`
	ClosePrice float32 `json:"close"`
	Volume     float32 `json:"volume"`
}

type MSIntradayData struct {
	Symbol    string  `json:"symbol"`
	Exchange  string  `json:"exchange"`
	PriceDate string  `json:"date"`
	LastPrice float32 `json:"last"`
	Volume    float32 `json:"volume"`
}

type MSEndOfDayData struct {
	Symbol        string         `json:"symbol"`
	Name          string         `json:"name"`
	StockExchange MSExchangeData `json:"stock_exchange"`
	EndOfDay      []MSIndexData  `json:"eod"`
}

type MSTickerData struct {
	Symbol        string         `json:"symbol"`
	Name          string         `json:"name"`
	StockExchange MSExchangeData `json:"stock_exchange"`
}

type MSEndOfDayResponse struct {
	Data MSEndOfDayData `json:"data"`
}

type MSExchangeResponse struct {
	Data []MSExchangeData `json:"data"`
}

type MSMarketIndexResponse struct {
	Data MSMarketIndexesData `json:"data"`
}

type MSIndexResponse struct {
	Data []MSIndexData `json:"data"`
}

type MSCurrencyResponse struct {
	Data []MSCurrencyData `json:"data"`
}

type MSIntradayResponse struct {
	Data []MSIntradayData `json:"data"`
}
type MSTickerResponse struct {
	Data []MSTickerData `json:"data"`
}

func getMarketstackData(awssess *session.Session, action string, params map[string]string) (string, string, *http.Response, error) {
	httpClient := http.Client{}

	api_access_key, err := myaws.AWSGetSecretKV(awssess, "marketstack", "api_access_key")
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
	for key, val := range params {
		q.Add(key, val)
	}
	logPath := req.URL.Path
	logQuery := q.Encode()

	q.Add("access_key", *api_access_key)
	req.URL.RawQuery = q.Encode()

	log.Info().
		Str("path", req.URL.Path).
		Str("query", logQuery).
		Msg("Making marketstack API request")

	res, err := httpClient.Do(req)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Failed to perform HTTP request")
	}
	if res.StatusCode != http.StatusOK {
		log.Fatal().
			Str("query", logQuery).
			Int("status_code", res.StatusCode).
			Msg("Failed to receive 200 success code from HTTP request")
	}

	return logPath, logQuery, res, nil
}

func fetchExchanges(awssess *session.Session, db *sqlx.DB) (int, error) {
	params := make(map[string]string)
	logPath, logQuery, res, err := getMarketstackData(awssess, "exchanges", params)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Failed to get marketstack data for exchanges")
	}

	defer res.Body.Close()

	var apiResponse MSExchangeResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	logData, err := json.Marshal(apiResponse.Data)
	recordHistoryS3(awssess, logPath, logQuery, string(logData))

	count := 0
	for _, MSExchangeData := range apiResponse.Data {
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

func fetchMarketIndexes(awssess *session.Session, db *sqlx.DB) (int, error) {
	params := make(map[string]string)
	logPath, logQuery, res, err := getMarketstackData(awssess, "exchanges/INDX/tickers", params)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Failed to get marketstack data for marketindexes")
	}

	defer res.Body.Close()

	var apiResponse MSMarketIndexResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	logData, err := json.Marshal(apiResponse.Data)
	recordHistoryS3(awssess, logPath, logQuery, string(logData))

	count := 0
	for _, MSMarketIndexData := range apiResponse.Data.MarketIndexes {
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

func fetchCurrencies(awssess *session.Session, db *sqlx.DB) (int, error) {
	params := make(map[string]string)
	logPath, logQuery, res, err := getMarketstackData(awssess, "currencies", params)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Failed to get marketstack data for currencies")
	}

	defer res.Body.Close()

	var apiResponse MSCurrencyResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	logData, err := json.Marshal(apiResponse.Data)
	recordHistoryS3(awssess, logPath, logQuery, string(logData))

	count := 0
	for _, MSCurrencyData := range apiResponse.Data {
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

func fetchTicker(awssess *session.Session, db *sqlx.DB, symbol string, exchangeMic string) (*Ticker, error) {
	params := make(map[string]string)
	params["limit"] = "31" // get, possibly, the full current month but let a scheduled job get everything else
	params["exchange"] = exchangeMic

	logPath, logQuery, res, err := getMarketstackData(awssess, fmt.Sprintf("tickers/%s/eod/latest", symbol), params)
	if err != nil {
		log.Error().Err(err).
			Str("symbol", symbol).
			Msg("Failed to get marketstack data for ticker")
		return nil, err
	}

	defer res.Body.Close()

	var apiResponse MSEndOfDayResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	logData, err := json.Marshal(apiResponse.Data)
	recordHistoryS3(awssess, logPath, logQuery, string(logData))

	var MSEndOfDayData MSEndOfDayData = apiResponse.Data

	// grab the exchange's countryId we'll need, create new record if needed
	var country = &Country{0, MSEndOfDayData.StockExchange.CountryCode, MSEndOfDayData.StockExchange.CountryName, "", ""}
	country, err = getOrCreateCountry(db, country)
	if err != nil {
		log.Error().Err(err).
			Str("country_code", MSEndOfDayData.StockExchange.CountryCode).
			Msg("Failed to create/update country for exchange")
		return nil, err
	}

	// grab the exchange_id we'll need, create new record if needed
	var exchange = &Exchange{0, MSEndOfDayData.StockExchange.Acronym, MSEndOfDayData.StockExchange.Mic, MSEndOfDayData.StockExchange.Name, country.CountryId, MSEndOfDayData.StockExchange.City, "", ""}
	exchange, err = getOrCreateExchange(db, exchange)
	if err != nil {
		log.Error().Err(err).
			Str("acronym", MSEndOfDayData.StockExchange.Acronym).
			Msg("Failed to create/update exchange")
		return nil, err
	}

	// use marketstack data to create or update ticker
	var ticker = &Ticker{0, MSEndOfDayData.Symbol, exchange.ExchangeId, MSEndOfDayData.Name, "", ""}
	ticker, err = createOrUpdateTicker(db, ticker)
	if err != nil {
		log.Error().Err(err).
			Str("symbol", MSEndOfDayData.Symbol).
			Str("acronym", MSEndOfDayData.StockExchange.Acronym).
			Msg("Failed to create/update ticker")
		return nil, err
	}

	// finally, lets roll through all the EOD price data we got and make sure we have it all
	var anyErr error
	for _, MSIndexData := range apiResponse.Data.EndOfDay {
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

	ticker.ScheduleEODUpdate(awssess, db)

	return ticker, nil
}

func fetchTickerIntraday(awssess *session.Session, db *sqlx.DB, ticker Ticker, exchange *Exchange, intradate string) error {
	params := make(map[string]string)
	params["symbols"] = ticker.TickerSymbol
	params["exchange"] = exchange.ExchangeMic
	params["interval"] = "5min"
	params["sort"] = "ASC"
	params["date_from"] = intradate + "T14:30:00Z" // 9:30 AM ET open
	params["date_to"] = intradate + "T21:05:00Z"   // 4:00 PM ET close

	logPath, logQuery, res, err := getMarketstackData(awssess, fmt.Sprintf("intraday/%s", intradate), params)
	if err != nil {
		log.Fatal().Err(err).
			Str("symbol", ticker.TickerSymbol).
			Str("intraday", intradate).
			Msg("Failed to get marketstack data for ticker")
	}

	defer res.Body.Close()

	var apiResponse MSIntradayResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	logData, err := json.Marshal(apiResponse.Data)
	recordHistoryS3(awssess, logPath, logQuery, string(logData))

	//log.Fatal().
	//	Str("response", fmt.Sprintf("%#v", apiResponse)).
	//	Msg("Failed to get valid marketstack data")

	// country isn't provided, exchange is but to do an intraday
	// we have to already have that info, so we'll skip doing
	// any updates for that from this API call

	// lets roll through all the intraday price data we got and make sure we have it all
	var anyErr error
	var priorVol float32
	for _, MSIntradayData := range apiResponse.Data {
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

func fetchMarketIndexIntraday(awssess *session.Session, db *sqlx.DB, marketindex MarketIndex, intradate string) error {
	params := make(map[string]string)
	params["symbols"] = marketindex.MarketIndexSymbol
	params["exchange"] = "INDX"
	params["interval"] = "5min"
	params["sort"] = "ASC"
	params["date_from"] = intradate + "T14:30:00Z" // 9:30 AM ET open
	params["date_to"] = intradate + "T21:05:00Z"   // 4:00 PM ET close

	logPath, logQuery, res, err := getMarketstackData(awssess, fmt.Sprintf("intraday/%s", intradate), params)
	if err != nil {
		log.Fatal().Err(err).
			Str("symbol", marketindex.MarketIndexSymbol).
			Str("intraday", intradate).
			Msg("Failed to get marketstack data for market index")
	}

	defer res.Body.Close()

	var apiResponse MSIntradayResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	logData, err := json.Marshal(apiResponse.Data)
	recordHistoryS3(awssess, logPath, logQuery, string(logData))

	//log.Fatal().
	//	Str("response", fmt.Sprintf("%#v", apiResponse)).
	//	Msg("Failed to get valid marketstack data")

	// country isn't provided, exchange is but to do an intraday
	// we have to already have that info, so we'll skip doing
	// any updates for that from this API call

	// lets roll through all the intraday price data we got and make sure we have it all
	var anyErr error
	var priorVol float32
	for _, MSIntradayData := range apiResponse.Data {
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

func jumpsearchMarketstackTicker(awssess *session.Session, db *sqlx.DB, searchString string) (SearchResult, error) {
	searchResults, err := listsearchMarketstackTicker(awssess, db, searchString)
	if err != nil {
		return SearchResult{}, err
	}
	if len(searchResults) > 0 {
		return searchResults[0], nil
	}
	return SearchResult{}, err
}

func listsearchMarketstackTicker(awssess *session.Session, db *sqlx.DB, searchString string) ([]SearchResult, error) {
	params := make(map[string]string)
	params["search"] = searchString

	logPath, logQuery, res, err := getMarketstackData(awssess, "tickers", params)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var apiResponse MSTickerResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	logData, err := json.Marshal(apiResponse.Data)
	recordHistoryS3(awssess, logPath, logQuery, string(logData))

	searchResults := make([]SearchResult, 0)
	for _, MSTickerData := range apiResponse.Data {
		searchResults = append(searchResults, SearchResult{MSTickerData.Symbol, MSTickerData.StockExchange.Acronym, MSTickerData.StockExchange.CountryName, MSTickerData.Name})
	}
	return searchResults, nil
}

func recordHistoryS3(awssess *session.Session, logPath string, logQuery string, logData string) {
	s3svc := s3.New(awssess)

	sha1Hash := sha1.New()
	io.WriteString(sha1Hash, logData)
	logKey := fmt.Sprintf("marketstack/%s/%x", logPath[1:], string(sha1Hash.Sum(nil)))

	inputPutObj := &s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(strings.NewReader(logData)),
		Bucket: aws.String("graystorm-stockwatch"),
		Key:    aws.String(logKey),
	}

	_, err := s3svc.PutObject(inputPutObj)
	if err != nil {
		log.Warn().Err(err).
			Str("bucket", "graystorm-stockwatch").
			Str("key", logKey).
			Msg("Failed to upload to S3 bucket")
	} else {
		log.Info().
			Str("bucket", "graystorm-stockwatch").
			Str("key", logKey).
			Msg("Stored marketstack reply to S3 bucket")
	}
}
