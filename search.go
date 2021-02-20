package main

import (
	"fmt"
	"net/http"

	"graystorm.com/mylog"
)

func searchHandler(w http.ResponseWriter, r *http.Request) {
	searchType := r.URL.Path[len("/search/"):]
	mylog.Info.Print("checking searchType")

	switch searchType {
	case "ticker":
		searchString := r.FormValue("searchString")
		if searchString == "" {
			errorHandler(w, r, "There was an empty search string")
			return
		}
		mylog.Info.Printf("doing search for searchString: %s", searchString)

		ticker, err := searchMarketstackTicker(searchString)
		if err != nil {
			errorHandler(w, r, fmt.Sprintf("Nothing found for search string: %s", searchString))
			return
		}

		exchange, err := getExchangeById(ticker.Exchange_id)
		if err != nil {
			errorHandler(w, r, fmt.Sprintf("An error occurred trying to get the exchange for symbol: %s", ticker.Ticker_symbol))
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/view/%s/%s", ticker.Ticker_symbol, exchange.Exchange_acronym), http.StatusFound)
		return
	default:
		mylog.Error.Printf("unknown update action: %s", searchType)
		http.NotFound(w, r)
	}

	var data = Message{""}
	renderTemplateMessages(w, r, "update", &data)
}
