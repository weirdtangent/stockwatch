package main

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/mytime"
)

func viewDailyHandler(aws *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)
		params := mux.Vars(r)
		symbol := params["symbol"]
		acronym := params["acronym"]

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

		ticker, _ = updateTickerWithEOD(aws, db, ticker)

		daily, err := getDailyMostRecent(db, ticker.Ticker_id)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.Ticker_symbol).
				Int64("ticker_id", ticker.Ticker_id).
				Msg("Failed to load most recent daily price for ticker")
			http.NotFound(w, r)
			return
		}
		lastDailyMove, err := getLastDailyMove(db, ticker.Ticker_id)
		if err != nil {
			lastDailyMove = "unknown"
		}

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

		var lineChartHTML = chartHandlerDailyLine(ticker, exchange, dailies, webwatches)
		var klineChartHTML = chartHandlerDailyKLine(ticker, exchange, dailies, webwatches)

		recents, err := addTickerToRecents(session, r, ticker.Ticker_symbol, exchange.Exchange_acronym)

		var Config = ConfigData{}
		renderTemplateDailyView(w, r, "view-daily",
			&TickerDailyView{Config, *ticker, *exchange, *daily, lastDailyMove, Dailies{dailies[len(dailies)-30:]}, webwatches, *recents, lineChartHTML, klineChartHTML})
	})
}

func viewIntradayHandler(aws *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)
		params := mux.Vars(r)
		symbol := params["symbol"]
		acronym := params["acronym"]
		intradate := params["intradate"]

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
			log.Warn().Err(err).
				Str("symbol", symbol).
				Int64("exchange_id", exchange.Exchange_id).
				Msg("Failed to find existing ticker")
			http.NotFound(w, r)
			return
		}

		updateTickerWithIntraday(aws, db, ticker, intradate)

		daily, err := getDailyMostRecent(db, ticker.Ticker_id)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.Ticker_symbol).
				Int64("ticker_id", ticker.Ticker_id).
				Msg("Failed to load most recent daily price for ticker")
			http.NotFound(w, r)
			return
		}
		lastDailyMove, err := getLastDailyMove(db, ticker.Ticker_id)
		if err != nil {
			lastDailyMove = "unknown"
		}

		// load up intradays for date selected
		intradays, err := loadIntradayData(db, ticker.Ticker_id, intradate)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.Ticker_symbol).
				Int64("ticker_id", ticker.Ticker_id).
				Str("intradate", intradate).
				Msg("Failed to load intraday prices for ticker")
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

		var lineChartHTML = chartHandlerIntradayLine(ticker, exchange, intradays, webwatches, intradate)

		recents, err := addTickerToRecents(session, r, ticker.Ticker_symbol, exchange.Exchange_acronym)

		var Config = ConfigData{}
		priorBusinessDay, _ := mytime.PriorBusinessDayStr(intradate + " 21:05:00")
		nextBusinessDay, _ := mytime.NextBusinessDayStr(intradate + " 13:55:00")
		log.Info().Str("prior", priorBusinessDay).Str("next", nextBusinessDay).Msg("")
		renderTemplateIntradayView(w, r, "view-intraday",
			&TickerIntradayView{Config, *ticker, *exchange, *daily, lastDailyMove, intradate, priorBusinessDay[0:10], nextBusinessDay[0:10], intradays, webwatches, *recents, lineChartHTML})
	})
}
