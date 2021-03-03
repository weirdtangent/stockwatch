package main

import (
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func searchHandler(awssess *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		searchType := params["type"]
		var messages = make([]Message, 0)

		switch searchType {
		case "ticker":
			searchString := r.FormValue("searchString")
			if searchString == "" {
				messages = append(messages, Message{fmt.Sprintf("Search text not entered"), "warning"})
				break
			}

			log.Info().
				Str("search_type", searchType).
				Str("search_string", searchString).
				Msg("Search performed")

			ticker, err := searchMarketstackTicker(awssess, db, searchString)
			if err != nil {
				messages = append(messages, Message{fmt.Sprintf("Sorry, error returned for that search"), "danger"})
				break
			}
			if ticker == nil {
				messages = append(messages, Message{fmt.Sprintf("Sorry, nothing found for '%s'", searchString), "warning"})
				break
			}

			exchange, err := getExchangeById(db, ticker.ExchangeId)
			if err != nil {
				log.Error().Msgf("An error occurred trying to get the exchange for symbol: %s", ticker.TickerSymbol)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/view/%s/%s", ticker.TickerSymbol, exchange.ExchangeAcronym), http.StatusFound)
			return
		default:
			log.Warn().
				Str("search_type", searchType).
				Msg("Unknown search_type")
			messages = append(messages, Message{fmt.Sprintf("Sorry, invalid search request"), "danger"})
		}

		session := getSession(r)
		recents, _ := getRecents(session, r)

		webdata := make(map[string]interface{})
		webdata["config"] = ConfigData{}
		webdata["recents"] = recents
		webdata["messages"] = Messages{messages}
		renderTemplateDefault(w, r, "home", webdata)
	})
}
