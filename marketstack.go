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

type MSIndexResponse struct {
	Data []MSIndexData `json:"data"`
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

func updateMarketstackExchanges(awssess *session.Session, db *sqlx.DB) (int, error) {
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
				break
			}

			// use marketstack data to create or update exchange
			var exchange = &Exchange{0, MSExchangeData.Acronym, MSExchangeData.Mic, MSExchangeData.Name, country.CountryId, MSExchangeData.City, "", ""}
			_, err = createOrUpdateExchange(db, exchange)
			if err != nil {
				log.Error().Err(err).
					Str("acronym", MSExchangeData.Acronym).
					Msg("Failed to create/update exchange")
				break
			}
			log.Info().Str("acronym", exchange.ExchangeAcronym).Msg("Exchange created/updated")
			count += 1
		}
	}
	log.Info().Int("count", count).Msg("Exchanges updated from marketstack")
	return count, nil
}

func updateMarketstackTicker(awssess *session.Session, db *sqlx.DB, symbol string) (*Ticker, error) {
	params := make(map[string]string)
	log.Info().Msgf("update ticker: %s", symbol)
	logPath, logQuery, res, err := getMarketstackData(awssess, fmt.Sprintf("tickers/%s/eod", symbol), params)
	if err != nil {
		log.Fatal().Err(err).
			Str("symbol", symbol).
			Msg("Failed to get marketstack data for ticker")
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
		log.Fatal().Err(err).
			Str("country_code", MSEndOfDayData.StockExchange.CountryCode).
			Msg("Failed to create/update country for exchange")
	}

	// grab the exchange_id we'll need, create new record if needed
	var exchange = &Exchange{0, MSEndOfDayData.StockExchange.Acronym, "", MSEndOfDayData.StockExchange.Name, country.CountryId, MSEndOfDayData.StockExchange.City, "", ""}
	exchange, err = getOrCreateExchange(db, exchange)
	if err != nil {
		log.Fatal().Err(err).
			Str("acronym", MSEndOfDayData.StockExchange.Acronym).
			Msg("Failed to create/update exchange")
	}

	// use marketstack data to create or update ticker
	var ticker = &Ticker{0, MSEndOfDayData.Symbol, exchange.ExchangeId, MSEndOfDayData.Name, "", ""}
	ticker, err = createOrUpdateTicker(db, ticker)
	if err != nil {
		log.Fatal().Err(err).
			Str("symbol", MSEndOfDayData.Symbol).
			Msg("Failed to create/update ticker")
	}

	// finally, lets roll through all the EOD price data we got and make sure we have it all
	for _, MSIndexData := range apiResponse.Data.EndOfDay {
		// use marketstack data to create or update dailies
		var daily = &Daily{0, ticker.TickerId, MSIndexData.PriceDate, MSIndexData.OpenPrice, MSIndexData.HighPrice, MSIndexData.LowPrice, MSIndexData.ClosePrice, MSIndexData.Volume, "", ""}
		if daily.Volume > 0 {
			_, err = createOrUpdateDaily(db, daily)
			if err != nil {
				log.Fatal().Err(err).
					Str("symbol", ticker.TickerSymbol).
					Int64("ticker_id", ticker.TickerId).
					Msg("Failed to create/update EOD for ticker")
			}
		}
	}

	return ticker, nil
}

func updateMarketstackIntraday(awssess *session.Session, db *sqlx.DB, ticker Ticker, exchange *Exchange, intradate string) error {
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
	var priorVol float32
	for _, MSIntradayData := range apiResponse.Data {
		timeSlot := MSIntradayData.PriceDate[11:16]
		if timeSlot >= "14:30" && timeSlot < "21:05" {
			// use marketstack data to create or update intradays
			var intraday = &Intraday{0, ticker.TickerId, MSIntradayData.PriceDate, MSIntradayData.LastPrice, MSIntradayData.Volume - priorVol, "", ""}
			_, err = createOrUpdateIntraday(db, intraday)
			if err != nil {
				log.Fatal().Err(err).
					Str("symbol", ticker.TickerSymbol).
					Int64("tickerId", ticker.TickerId).
					Msg("Failed to create/update Intraday for ticker")
			}
			priorVol = MSIntradayData.Volume
		}
	}

	return nil
}

func searchMarketstackTicker(awssess *session.Session, db *sqlx.DB, search string) (*Ticker, error) {
	params := make(map[string]string)
	params["search"] = search

	logPath, logQuery, res, err := getMarketstackData(awssess, "tickers", params)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var apiResponse MSTickerResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	logData, err := json.Marshal(apiResponse.Data)
	recordHistoryS3(awssess, logPath, logQuery, string(logData))

	for _, MSTickerData := range apiResponse.Data {
		if MSTickerData.Symbol != "" {
			// grab the exchange's countryId we'll need, create new record if needed
			var country = &Country{0, MSTickerData.StockExchange.CountryCode, MSTickerData.StockExchange.CountryName, "", ""}
			country, err = getOrCreateCountry(db, country)
			if err != nil {
				log.Fatal().Err(err).
					Str("country_code", MSTickerData.StockExchange.CountryCode).
					Msg("Failed to create/update country for exchange")
			}

			// grab the exchange_id we'll need, create new record if needed
			var exchange = &Exchange{0, MSTickerData.StockExchange.Acronym, "", MSTickerData.StockExchange.Name, country.CountryId, MSTickerData.StockExchange.City, "", ""}
			exchange, err = getOrCreateExchange(db, exchange)
			if err != nil {
				log.Fatal().Err(err).
					Str("acronym", MSTickerData.StockExchange.Acronym).
					Msg("Failed to create/update exchange")
			}

			// use marketstack data to create or update ticker
			var ticker = &Ticker{0, MSTickerData.Symbol, exchange.ExchangeId, MSTickerData.Name, "", ""}
			ticker, err = createOrUpdateTicker(db, ticker)
			if err != nil {
				return nil, err
			}
			return updateMarketstackTicker(awssess, db, ticker.TickerSymbol)
		}
	}

	return nil, err
}

func recordHistoryS3(awssess *session.Session, logPath string, logQuery string, logData string) {
	s3svc := s3.New(awssess)

	sha1Hash := sha1.New()
	io.WriteString(sha1Hash, logPath+"?"+logQuery)
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
	}
}
