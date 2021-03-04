package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/mymath"
	"github.com/weirdtangent/mytime"
)

func viewDailyHandler(awssess *session.Session, db *sqlx.DB, sc *securecookie.SecureCookie) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := make(map[string]interface{})
		messages := make([]Message, 0)
		session := getSession(r)
		if ok := checkAuthState(w, r, db, sc, webdata); ok == false {
			http.Redirect(w, r, "/", 401)
		}

		params := mux.Vars(r)
		symbol := params["symbol"]
		acronym := params["acronym"]

		timespan := 90
		if tsParam := r.FormValue("ts"); tsParam != "" {
			if tsValue, err := strconv.ParseInt(tsParam, 10, 32); err == nil {
				timespan = int(mymath.MinMax(tsValue, 15, 180))
			} else if err != nil {
				log.Error().Err(err).Str("ts", tsParam).Msg("Failed to interpret timespan (ts) param")
			}
			log.Info().Int("timespan", timespan).Msg("")
		}

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
			ticker, err = updateMarketstackTicker(awssess, db, symbol)
			if err != nil {
				log.Warn().Err(err).
					Str("symbol", symbol).
					Msg("Failed to update EOD for ticker")
				http.NotFound(w, r)
				return
			}
		}

		updated, err := ticker.updateDailies(awssess, db)
		if err != nil {
			messages = append(messages, Message{fmt.Sprintf("Failed to update End-of-day data for %s", ticker.TickerSymbol), "danger"})
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("ticker_id", ticker.TickerId).
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
				Int64("ticker_id", ticker.TickerId).
				Msg("Failed to load most recent daily price for ticker")
		}
		lastDailyMove, err := getLastDailyMove(db, ticker.TickerId)
		if err != nil {
			lastDailyMove = "unknown"
		}

		// load up to last 100 days of EOD data
		dailies, err := ticker.LoadDailies(db, timespan)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int("timespan", timespan).
				Int64("ticker_id", ticker.TickerId).
				Msg("Failed to load daily prices for ticker")
			http.NotFound(w, r)
			return
		}

		// load any active watches about this ticker
		webwatches, err := loadWebWatches(db, ticker.TickerId)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("ticker_id", ticker.TickerId).
				Msg("Failed to load watches for ticker")
			http.NotFound(w, r)
			return
		}

		var lineChartHTML = chartHandlerDailyLine(ticker, exchange, dailies, webwatches)
		var klineChartHTML = chartHandlerDailyKLine(ticker, exchange, dailies, webwatches)

		recents, err := addTickerToRecents(session, r, ticker.TickerSymbol, exchange.ExchangeAcronym)

		webdata["recents"] = Recents{*recents}
		webdata["messages"] = Messages{messages}
		webdata["ticker"] = ticker
		webdata["exchange"] = exchange
		webdata["timespan"] = timespan
		webdata["daily"] = daily
		webdata["lastDailyMove"] = lastDailyMove
		webdata["dailies"] = Dailies{dailies}
		webdata["watches"] = webwatches
		webdata["lineChart"] = lineChartHTML
		webdata["klineChart"] = klineChartHTML

		renderTemplateDefault(w, r, "view-daily", webdata)
	})
}

func viewIntradayHandler(awssess *session.Session, db *sqlx.DB, sc *securecookie.SecureCookie) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := make(map[string]interface{})
		messages := make([]Message, 0)
		session := getSession(r)
		if ok := checkAuthState(w, r, db, sc, webdata); ok == false {
			http.Redirect(w, r, "/", 401)
		}

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
		ticker, err := getTicker(db, symbol, exchange.ExchangeId)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", symbol).
				Int64("exchange_id", exchange.ExchangeId).
				Msg("Failed to find existing ticker")
			http.NotFound(w, r)
			return
		}

		updated, err := ticker.updateIntradays(awssess, db, intradate)
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

		webdata["recents"] = Recents{*recents}
		webdata["messages"] = Messages{messages}
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
