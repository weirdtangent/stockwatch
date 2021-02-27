package main

import (
	"fmt"
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
		var messages = make([]Message, 0)

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
		ticker, err := getTicker(db, symbol, exchange.ExchangeId)
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

		updated, err := ticker.updateDailies(aws, db)
		if err != nil {
			messages = append(messages, Message{fmt.Sprintf("Failed to update End-of-day data for %s", ticker.TickerSymbol), "danger"})
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Msg("Failed to update EOD for ticker")
		}
		if updated {
			messages = append(messages, Message{fmt.Sprintf("End-of-day data updated for %s", ticker.TickerSymbol), "success"})
		}

		daily, err := getDailyMostRecent(db, ticker.TickerId)
		if err != nil {
			messages = append(messages, Message{fmt.Sprintf("No End-of-day data found for %s", ticker.TickerSymbol), "warning"})
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Msg("Failed to load most recent daily price for ticker")
		}
		lastDailyMove, err := getLastDailyMove(db, ticker.TickerId)
		if err != nil {
			lastDailyMove = "unknown"
		}

		// load up to last 100 days of EOD data
		dailies, err := ticker.LoadDailies(db, 100)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Msg("Failed to load daily prices for ticker")
			http.NotFound(w, r)
			return
		}

		// load any active watches about this ticker
		webwatches, err := loadWebWatches(db, ticker.TickerId)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Msg("Failed to load watches for ticker")
			http.NotFound(w, r)
			return
		}

		var lineChartHTML = chartHandlerDailyLine(ticker, exchange, dailies, webwatches)
		var klineChartHTML = chartHandlerDailyKLine(ticker, exchange, dailies, webwatches)

		recents, err := addTickerToRecents(session, r, ticker.TickerSymbol, exchange.ExchangeAcronym)

		var tableDailies Dailies
		if len(dailies) > 0 {
			if len(dailies) <= 30 {
				tableDailies.Days = dailies[len(dailies)-30:]
			} else {
				tableDailies.Days = dailies
			}
		}

		webdata := make(map[string]interface{})
		webdata["config"] = ConfigData{}
		webdata["recents"] = recents
		webdata["messages"] = messages

		webdata["ticker"] = ticker
		webdata["exchange"] = exchange
		webdata["daily"] = daily
		webdata["lastDailyMove"] = lastDailyMove
		webdata["dailies"] = tableDailies
		webdata["watches"] = webwatches
		webdata["lineChart"] = lineChartHTML
		webdata["klineChart"] = klineChartHTML

		renderTemplateDefault(w, r, "view-daily", webdata)
	})
}

func viewIntradayHandler(aws *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)
		params := mux.Vars(r)
		symbol := params["symbol"]
		acronym := params["acronym"]
		intradate := params["intradate"]
		var messages = make([]Message, 0)

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
		ticker, err := getTicker(db, symbol, exchange.ExchangeId)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", symbol).
				Int64("exchange_id", exchange.ExchangeId).
				Msg("Failed to find existing ticker")
			http.NotFound(w, r)
			return
		}

		updated, err := ticker.updateIntradays(aws, db, intradate)
		if err != nil {
			messages = append(messages, Message{fmt.Sprintf("Failed to update intraday data for %s for %s", ticker.TickerSymbol, intradate), "danger"})
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Str("intradate", intradate).
				Msg("Failed to update intrday for ticker")
		}
		if updated {
			messages = append(messages, Message{fmt.Sprintf("Intraday data updated for %s for %s", ticker.TickerSymbol, intradate), "success"})
		}

		daily, err := getDailyMostRecent(db, ticker.TickerId)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Msg("Failed to load most recent daily price for ticker")
			http.NotFound(w, r)
			return
		}
		lastDailyMove, err := getLastDailyMove(db, ticker.TickerId)
		if err != nil {
			lastDailyMove = "unknown"
		}

		// load up intradays for date selected
		intradays, err := ticker.LoadIntraday(db, intradate)
		if err != nil {
			messages = append(messages, Message{fmt.Sprintf("No Intraday data found for %s", ticker.TickerSymbol), "warning"})
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Str("intradate", intradate).
				Msg("Failed to load intraday prices for ticker")
			http.NotFound(w, r)
			return
		}
		if len(intradays) < 20 {
			messages = append(messages, Message{fmt.Sprintf("No Intraday data found for %s", ticker.TickerSymbol), "warning"})
			intradays = []Intraday{}
		}

		// load any active watches about this ticker
		webwatches, err := loadWebWatches(db, ticker.TickerId)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Msg("Failed to load watches for ticker")
			http.NotFound(w, r)
			return
		}

		var lineChartHTML = chartHandlerIntradayLine(ticker, exchange, intradays, webwatches, intradate)

		recents, err := addTickerToRecents(session, r, ticker.TickerSymbol, exchange.ExchangeAcronym)

		priorBusinessDay, _ := mytime.PriorBusinessDayStr(intradate + " 21:05:00")
		nextBusinessDay, _ := mytime.NextBusinessDayStr(intradate + " 13:55:00")
		log.Info().Str("prior", priorBusinessDay).Str("next", nextBusinessDay).Msg("")

		webdata := make(map[string]interface{})
		webdata["config"] = ConfigData{}
		webdata["recents"] = recents
		webdata["messages"] = messages

		webdata["ticker"] = ticker
		webdata["exchange"] = exchange
		webdata["daily"] = daily
		webdata["lastDailyMove"] = lastDailyMove
		webdata["intradate"] = intradate
		webdata["priorBusinessDate"] = priorBusinessDay[0:10]
		webdata["nextBusinessDate"] = nextBusinessDay[0:10]
		webdata["intradays"] = Intradays{intradays}
		webdata["watches"] = webwatches
		webdata["lineChart"] = lineChartHTML
		renderTemplateDefault(w, r, "view-intraday", webdata)
	})
}
