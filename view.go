package main

import (
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func viewHandler(aws *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)
		path_paramlist := r.URL.Path[len("/view/"):]
		params := strings.Split(path_paramlist, "/")
		symbol := params[0]
		acronym := params[1]

		// grab exchange they asked for
		exchange, err := getExchange(db, acronym)
		if err != nil {
			log.Warn().Err(err).
				Str("acronym", acronym).
				Msg("Invalid table key")
			http.NotFound(w, r)
			return
		}

		// find ticker specifically at that exchange (since there are overlaps)
		ticker, err := getTicker(db, symbol, exchange.Exchange_id)
		if err != nil {
			ticker, err = updateMarketstackTicker(aws, db, symbol)
			if err != nil {
				log.Warn().Err(err).
					Str("symbol", symbol).
					Msg("Failed to update EOD for ticker")
				http.NotFound(w, r)
				return
			}
		}

		ticker, _ = updateTicker(aws, db, ticker)

		// load up to last 100 days of EOD data
		dailies, err := loadDailies(db, ticker.Ticker_id, 100)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.Ticker_symbol).
				Int64("ticker_id", ticker.Ticker_id).
				Msg("Failed to load daily prices for ticker")
			http.NotFound(w, r)
			return
		}

		// load any active watches about this ticker
		webwatches, err := loadWebWatches(db, ticker.Ticker_id)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.Ticker_symbol).
				Int64("ticker_id", ticker.Ticker_id).
				Msg("Failed to load watches for ticker")
			http.NotFound(w, r)
			return
		}

		var lineChartHTML = chartHandlerLine(ticker, exchange, dailies, webwatches)
		var klineChartHTML = chartHandlerKLine(ticker, exchange, dailies, webwatches)

		recents, err := addTickerToRecents(session, r, ticker.Ticker_symbol, exchange.Exchange_acronym)

		var Config = ConfigData{}
		renderTemplateView(w, r, "view", &TickerView{Config, *ticker, *exchange, dailies[1:30], webwatches, *recents, lineChartHTML, klineChartHTML})
	})
}
