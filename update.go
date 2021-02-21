package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"graystorm.com/mylog"
)

func updateHandler(w http.ResponseWriter, r *http.Request) {
	path_paramlist := r.URL.Path[len("/update/"):]
	params := strings.Split(path_paramlist, "/")
	action := params[0]

	switch action {
	case "exchanges":
		mylog.Info.Print("ok doing update of exchanges")
		success, err := updateMarketstackExchanges()
		if err != nil {
			errorHandler(w, r, fmt.Sprintf("Bulk update of Exchanges failed: %s", err))
			return
		}
		if success != true {
			errorHandler(w, r, "Bulk update of Exchanges failed")
			return
		}
	case "ticker":
		symbol := params[1]
		_, err := updateMarketstackTicker(symbol)
		if err != nil {
			errorHandler(w, r, fmt.Sprintf("Update of ticket symbol %s failed: %s", symbol, err))
			return
		}
	case "dummy":
		mylog.Info.Print("just show the template")
	default:
		mylog.Error.Fatal("unknown update action: " + action)
	}

	errorHandler(w, r, "Operation completed normally")
}

// see if we need to pull a daily update:
//  if we don't have the EOD price for the prior business day
//  OR if we don't have it for the current business day and it's now 7pm or later
func updateTicker(ticker *Ticker) (*Ticker, error) {
	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentTime := time.Now().In(EasternTZ)
	priorWeekDay := currentTime.AddDate(0, 0, -1)
	for ; priorWeekDay.Weekday() == 0 || priorWeekDay.Weekday() == 6; priorWeekDay.AddDate(0, 0, -1) {
	}

	time_now := currentTime.Format("15:04:05")
	date_now := currentTime.Format("2006-01-02")
	is_today_weekday := (currentTime.Weekday() > 0 && currentTime.Weekday() < 6)

	date_prior := priorWeekDay.Format("2006-01-02")
	most_recent, err := getDailyMostRecent(ticker.Ticker_id)

	// if we don't have most recent EOD prices up to the previous workday, get em
	// or, if we don't have today's AND today is a weekday AND it's past 7PM, get em
	if err == nil && (most_recent.Price_date < date_prior || (most_recent.Price_date < date_now && is_today_weekday && time_now > "19:00:00")) {
		mylog.Warning.Printf("Use Marketstack API to get the latest EOD price info for %s", ticker.Ticker_symbol)
		ticker, err = updateMarketstackTicker(ticker.Ticker_symbol)
		if err == nil {
			mylog.Info.Printf("%s updated with latest EOD prices", ticker.Ticker_symbol)
		}
	}

	return ticker, err
}
