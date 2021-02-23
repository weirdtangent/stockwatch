package main

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

func searchHandler(w http.ResponseWriter, r *http.Request) {
	searchType := r.URL.Path[len("/search/"):]

	switch searchType {
	case "ticker":
		searchString := r.FormValue("searchString")
		if searchString == "" {
			errorHandler(w, r, "There was an empty search string")
			return
		}

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
		log.Warn().Str("searchtype", searchType).Msg("Unknown search_type")
		http.NotFound(w, r)
	}

	var data = Message{Config, ""}
	renderTemplateMessages(w, r, "update", &data)
}
