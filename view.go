package main

import (
	"net/http"
	"strings"
	"time"

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

	// see if we need to pull a daily update:
	//  if we don't have the EOD price for the prior business day
	//  OR if we don't have it for the current business day and it's now 7pm or later
	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentTime := time.Now().In(EasternTZ)
	priorWeekDay := currentTime.AddDate(0, 0, -1)
	for ; priorWeekDay.Weekday() == 0 || priorWeekDay.Weekday() == 6; priorWeekDay.AddDate(0, 0, -1) {
	}

	date_prior := priorWeekDay.Format("2006-01-2")
	time_now := currentTime.Format("15:04:05")
	most_recent, err := getDailyMostRecent(ticker.Ticker_id)

	if err == nil && (most_recent.Price_date < date_prior || (most_recent.Price_date == date_prior && time_now > "19:00:00")) {
		mylog.Warning.Printf("Use Marketstack API to get the latest EOD price info for %s", ticker.Ticker_symbol)
		updateMarketstackTicker(symbol)
	}

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
	var data = TickerView{*ticker, *exchange, dailies[1:30], webwatches, lineChartHTML, klineChartHTML}

	renderTemplateView(w, r, "view", &data)
}
