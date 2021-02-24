package main

import (
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func searchHandler(aws *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchType := r.URL.Path[len("/search/"):]

		switch searchType {
		case "ticker":
			searchString := r.FormValue("searchString")
			if searchString == "" {
				log.Error().Msg("There was an empty search string")
				return
			}

			log.Info().
				Str("search_type", searchType).
				Str("search_string", searchString).
				Msg("Unknown search_type")
			ticker, err := searchMarketstackTicker(aws, db, searchString)
			if err != nil {
				log.Error().Msgf("Nothing found for search string: %s", searchString)
				return
			}

			exchange, err := getExchangeById(db, ticker.Exchange_id)
			if err != nil {
				log.Error().Msgf("An error occurred trying to get the exchange for symbol: %s", ticker.Ticker_symbol)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/view/%s/%s", ticker.Ticker_symbol, exchange.Exchange_acronym), http.StatusFound)
			return
		default:
			log.Warn().
				Str("search_type", searchType).
				Msg("Unknown search_type")
			http.NotFound(w, r)
		}

		var data = Message{Config: ConfigData{}, MessageText: ""}
		renderTemplateMessages(w, r, "update", &data)
	})
}
