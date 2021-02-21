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
	mostRecentDaily, err := getDailyMostRecent(ticker.Ticker_id)
  if err != nil {
    mylog.Warning.Printf("Error getting most recent EOD date for %s(%d): %s", ticker.Ticker_symbol, ticker.Ticker_id, err)
    return ticker, err
  }
  mostRecentDailyDate := mostRecentDaily.Price_date
  mostRecentAvailable := mostRecentEODPricesAvailable()

	if mostRecentDailyDate < mostRecentAvailable {
		mylog.Info.Printf("Using Marketstack API to get the latest EOD price info for %s(%d)", ticker.Ticker_symbol, ticker.Ticker_id)
		ticker, err = updateMarketstackTicker(ticker.Ticker_symbol)
		if err != nil {
      mylog.Warning.Print("Error getting EOD prices for %s(%d): %s", ticker.Ticker_symbol, ticker.Ticker_id, err)
      return ticker, err
    }
		mylog.Info.Printf("%s(%d) updated with latest EOD prices", ticker.Ticker_symbol, ticker.Ticker_id)
	}

	return ticker, nil
}

func mostRecentEODPricesAvailable() string {
	currentDateTime := time.Local
	currentTime := currentDateTime.Format("15:04:05")
	currentDate := currentDateTime.Format("2006-01-02")
  isWorkDay := mytime.isWorkday(currentDateTime)

  if isWorkDay && currentTime > "19:00:00" {
    return currentDate
  }

  prevWorkDate := mytime.lastWorkDate(currentDateTime)
  prevWorkDay := prevWorkDate.Format("2006-01-02")

  return prevWorkDay
}

