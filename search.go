package main

import (
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func searchHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		awssess := ctx.Value("awssess").(*session.Session)
		db := ctx.Value("db").(*sqlx.DB)
		webdata := ctx.Value("webdata").(map[string]interface{})

		params := mux.Vars(r)
		searchType := params["type"]
		var messages = make([]Message, 0)

		if ok := checkAuthState(w, r); ok == false {
			encoded, err := encryptURL(awssess, ([]byte(r.URL.String())))
			if err == nil {
				http.Redirect(w, r, "/?next="+string(encoded), 302)
			} else {
				http.Redirect(w, r, "/", 302)
			}
			return
		}

		switch searchType {
		case "ticker":
			searchString := r.FormValue("searchString")
			if searchString == "" {
				messages = append(messages, Message{fmt.Sprintf("Search text not entered"), "warning"})
				break
			}

			logger.Info().
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
				logger.Error().Msgf("An error occurred trying to get the exchange for symbol: %s", ticker.TickerSymbol)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/view/%s/%s", ticker.TickerSymbol, exchange.ExchangeAcronym), http.StatusFound)
			return
		default:
			logger.Warn().
				Str("search_type", searchType).
				Msg("Unknown search_type")
			messages = append(messages, Message{fmt.Sprintf("Sorry, invalid search request"), "danger"})
		}

		webdata["messages"] = Messages{messages}
		renderTemplateDefault(w, r, "home", webdata)
	})
}
