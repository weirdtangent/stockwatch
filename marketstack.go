package stockwatch

import (
	"encoding/json"
	"fmt"
	"net/http"

	"graystorm.com/myaws"
	"graystorm.com/mylog"
)

func getMarketstackData(action string) (*http.Response, error) {
	httpClient := http.Client{}

	api_access_key, err := myaws.AWSGetSecretKV(aws_session, "marketstack", "api_access_key")
	if err != nil {
		mylog.Error.Fatal(err)
	}

	req, err := http.NewRequest("GET", "http://api.marketstack.com/v1/"+action, nil)
	if err != nil {
		mylog.Error.Fatal(err)
	}

	q := req.URL.Query()
	q.Add("access_key", *api_access_key)
	req.URL.RawQuery = q.Encode()

	res, err := httpClient.Do(req)
	if err != nil {
		mylog.Error.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		mylog.Error.Fatal(fmt.Sprintf("Non-success to %s\nHTTP response code %d", req.URL, res.StatusCode))
	}

	return res, nil
}

func updateMarketstackExchanges() (bool, error) {
	res, err := getMarketstackData("exchanges")
	if err != nil {
		mylog.Error.Fatal(err)
	}

	defer res.Body.Close()

	var apiResponse MSExchangeResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	for _, MSExchangeData := range apiResponse.Data {
		if MSExchangeData.Acronym != "" {
			// grab the country_id we'll need, create new record if needed
			var country = &Country{0, MSExchangeData.Country_code, MSExchangeData.Country, "", ""}
			country, err := createOrUpdateCountry(country)
			if err != nil {
				mylog.Error.Fatal(err)
			}

			// use marketstack data to create or update exchange
			var exchange = &Exchange{0, MSExchangeData.Acronym, MSExchangeData.Name, country.Country_id, MSExchangeData.City, "", ""}
			_, err = createOrUpdateExchange(exchange)
			if err != nil {
				mylog.Error.Fatal(err)
			}
		}
	}
	return true, nil
}

func updateMarketstackTicker(symbol string) (*Ticker, error) {
	res, err := getMarketstackData(fmt.Sprintf("tickers/%s/eod", symbol))
	if err != nil {
		mylog.Error.Fatal(err)
	}

	defer res.Body.Close()

	var apiResponse MSEndOfDayResponse
	json.NewDecoder(res.Body).Decode(&apiResponse)

	var MSEndOfDayData MSEndOfDayData = apiResponse.Data

	// grab the exchange's country_id we'll need, create new record if needed
	var country = &Country{0, MSEndOfDayData.StockExchange.Country_code, MSEndOfDayData.StockExchange.Country, "", ""}
	country, err = getOrCreateCountry(country)
	if err != nil {
		mylog.Error.Fatal(err)
	}

	// grab the exchange_id we'll need, create new record if needed
	var exchange = &Exchange{0, MSEndOfDayData.StockExchange.Acronym, MSEndOfDayData.StockExchange.Name, country.Country_id, MSEndOfDayData.StockExchange.City, "", ""}
	exchange, err = getOrCreateExchange(exchange)
	if err != nil {
		mylog.Error.Fatal(err)
	}

	// use marketstack data to create or update ticker
	var ticker = &Ticker{0, MSEndOfDayData.Symbol, exchange.Exchange_id, MSEndOfDayData.Name, "", ""}
	ticker, err = createOrUpdateTicker(ticker)
	if err != nil {
		mylog.Error.Fatal(err)
	}

	// finally, lets roll through all the EOD price data we got and make sure we have it all
	for _, MSIndexData := range apiResponse.Data.EndOfDay {
		// use marketstack data to create or update dailies
		var daily = &Daily{0, ticker.Ticker_id, MSIndexData.Price_date, MSIndexData.Open_price, MSIndexData.High_price, MSIndexData.Low_price, MSIndexData.Close_price, MSIndexData.Volume, "", ""}
		if daily.Volume > 0 {
			_, err = createOrUpdateDaily(daily)
			if err != nil {
				mylog.Error.Fatal(err)
			}
		}
	}

	return ticker, nil
}
