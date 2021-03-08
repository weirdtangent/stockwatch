package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/mymath"
	"github.com/weirdtangent/mytime"
)

func viewDailyHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		awssess := ctx.Value("awssess").(*session.Session)
		db := ctx.Value("db").(*sqlx.DB)
		webdata := ctx.Value("webdata").(map[string]interface{})
		messages := ctx.Value("messages").(*[]Message)

		session := getSession(r)
		if ok := checkAuthState(w, r); ok == false {
			encoded, err := encryptURL(awssess, ([]byte(r.URL.String())))
			if err == nil {
				http.Redirect(w, r, "/?next="+string(encoded), 302)
			} else {
				http.Redirect(w, r, "/", 302)
			}
			return
		}

		params := mux.Vars(r)
		symbol := params["symbol"]
		acronym := params["acronym"]

		timespan := 90
		if tsParam := r.FormValue("ts"); tsParam != "" {
			if tsValue, err := strconv.ParseInt(tsParam, 10, 32); err == nil {
				timespan = int(mymath.MinMax(tsValue, 15, 180))
			} else if err != nil {
				logger.Error().Err(err).Str("ts", tsParam).Msg("Failed to interpret timespan (ts) param")
			}
			logger.Info().Int("timespan", timespan).Msg("")
		}

		// grab exchange they asked for
		exchange, err := getExchange(db, acronym)
		if err != nil {
			logger.Warn().Err(err).
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
				logger.Warn().Err(err).
					Str("symbol", symbol).
					Msg("Failed to update EOD for ticker")
				http.NotFound(w, r)
				return
			}
		}

		updated, err := ticker.updateDailies(ctx)
		if err != nil {
			*messages = append(*messages, Message{fmt.Sprintf("Failed to update End-of-day data for %s", ticker.TickerSymbol), "danger"})
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("ticker_id", ticker.TickerId).
				Msg("Failed to update EOD for ticker")
		}
		if updated {
			*messages = append(*messages, Message{fmt.Sprintf("End-of-day data updated for %s", ticker.TickerSymbol), "success"})
		}

		daily, err := getDailyMostRecent(db, ticker.TickerId)
		if err != nil {
			*messages = append(*messages, Message{fmt.Sprintf("No End-of-day data found for %s", ticker.TickerSymbol), "warning"})
			logger.Warn().Err(err).
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
			logger.Warn().Err(err).
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
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("ticker_id", ticker.TickerId).
				Msg("Failed to load watches for ticker")
			http.NotFound(w, r)
			return
		}

		var lineChartHTML = chartHandlerDailyLine(ctx, ticker, exchange, dailies, webwatches)
		var klineChartHTML = chartHandlerDailyKLine(ctx, ticker, exchange, dailies, webwatches)

		recents, err := addTickerToRecents(session, r, ticker.TickerSymbol, exchange.ExchangeAcronym)

		webdata["recents"] = Recents{*recents}
		webdata["ticker"] = ticker
		webdata["exchange"] = exchange
		webdata["timespan"] = timespan
		webdata["daily"] = daily
		webdata["lastDailyMove"] = lastDailyMove
		webdata["dailies"] = Dailies{dailies}
		webdata["watches"] = webwatches
		webdata["lineChart"] = lineChartHTML
		webdata["klineChart"] = klineChartHTML

		renderTemplateDefault(w, r, "view-daily")
	})
}

func viewIntradayHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		awssess := ctx.Value("awssess").(*session.Session)
		db := ctx.Value("db").(*sqlx.DB)
		webdata := ctx.Value("webdata").(map[string]interface{})
		messages := ctx.Value("messages").(*[]Message)

		session := getSession(r)
		if ok := checkAuthState(w, r); ok == false {
			encoded, err := encryptURL(awssess, ([]byte(r.URL.String())))
			if err == nil {
				http.Redirect(w, r, "/?next="+string(encoded), 302)
			} else {
				http.Redirect(w, r, "/", 302)
			}
			return
		}

		params := mux.Vars(r)
		symbol := params["symbol"]
		acronym := params["acronym"]
		intradate := params["intradate"]

		// grab exchange they asked for
		exchange, err := getExchange(db, acronym)
		if err != nil {
			logger.Warn().Err(err).
				Str("acronym", acronym).
				Msg("Invalid table key")
			http.NotFound(w, r)
			return
		}

		// find ticker specifically at that exchange (since there are overlaps)
		ticker, err := getTicker(db, symbol, exchange.ExchangeId)
		if err != nil {
			logger.Warn().Err(err).
				Str("symbol", symbol).
				Int64("exchange_id", exchange.ExchangeId).
				Msg("Failed to find existing ticker")
			http.NotFound(w, r)
			return
		}

		updated, err := ticker.updateIntradays(ctx, intradate)
		if err != nil {
			*messages = append(*messages, Message{fmt.Sprintf("Failed to update intraday data for %s for %s", ticker.TickerSymbol, intradate), "danger"})
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Str("intradate", intradate).
				Msg("Failed to update intrday for ticker")
		}
		if updated {
			*messages = append(*messages, Message{fmt.Sprintf("Intraday data updated for %s for %s", ticker.TickerSymbol, intradate), "success"})
		}

		daily, err := getDailyMostRecent(db, ticker.TickerId)
		if err != nil {
			logger.Warn().Err(err).
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
			*messages = append(*messages, Message{fmt.Sprintf("No Intraday data found for %s", ticker.TickerSymbol), "warning"})
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Str("intradate", intradate).
				Msg("Failed to load intraday prices for ticker")
			http.NotFound(w, r)
			return
		}
		if len(intradays) < 20 {
			*messages = append(*messages, Message{fmt.Sprintf("No Intraday data found for %s", ticker.TickerSymbol), "warning"})
			intradays = []Intraday{}
		}

		// load any active watches about this ticker
		webwatches, err := loadWebWatches(db, ticker.TickerId)
		if err != nil {
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Msg("Failed to load watches for ticker")
			http.NotFound(w, r)
			return
		}

		var lineChartHTML = chartHandlerIntradayLine(ctx, ticker, exchange, intradays, webwatches, intradate)

		recents, err := addTickerToRecents(session, r, ticker.TickerSymbol, exchange.ExchangeAcronym)

		priorBusinessDay, _ := mytime.PriorBusinessDayStr(intradate + " 21:05:00")
		nextBusinessDay, _ := mytime.NextBusinessDayStr(intradate + " 13:55:00")
		logger.Info().Str("prior", priorBusinessDay).Str("next", nextBusinessDay).Msg("")

		webdata["recents"] = Recents{*recents}
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
		renderTemplateDefault(w, r, "view-intraday")
	})
}
