package main

import (
	"net/http"
	"strings"

	"graystorm.com/mylog"
)

func viewHandler(w http.ResponseWriter, r *http.Request) {
	path_paramlist := r.URL.Path[len("/view/"):]
	params := strings.Split(path_paramlist, "/")
	symbol := params[0]
	acronym := params[1]

	// grab exchange they asked for
	exchange, err := getExchange(acronym)
	if err != nil {
		mylog.Warning.Printf("Invalid acronym: %s. %s", acronym, err)
		http.NotFound(w, r)
		return
	}

	// find ticker specifically at that exchange (since there are overlaps)
	ticker, err := getTicker(symbol, exchange.Exchange_id)
	if err != nil {
		ticker, err = updateMarketstackTicker(symbol)
		if err != nil {
			mylog.Warning.Print(err)
			http.NotFound(w, r)
			return
		}
	}

	ticker, _ = updateTicker(ticker)

	// load up to last 100 days of EOD data
	dailies, err := loadDailies(ticker.Ticker_id, 100)
	if err != nil {
		mylog.Warning.Print(err)
		http.NotFound(w, r)
		return
	}

	// load any active watches about this ticker
	webwatches, err := loadWebWatches(ticker.Ticker_id)
	if err != nil {
		mylog.Warning.Print(err)
		http.NotFound(w, r)
		return
	}

	var lineChartHTML = chartHandlerLine(ticker, exchange, dailies, webwatches)
	var klineChartHTML = chartHandlerKLine(ticker, exchange, dailies, webwatches)

	recent, err := addTickerToRecent(r, ticker.Ticker_symbol, exchange.Exchange_acronym)

	renderTemplateView(w, r, "view", &TickerView{*ticker, *exchange, dailies[1:30], webwatches, *recent, lineChartHTML, klineChartHTML})
}
