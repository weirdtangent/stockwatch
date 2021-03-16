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

func viewTickerDailyHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		awssess := ctx.Value("awssess").(*session.Session)
		webdata := ctx.Value("webdata").(map[string]interface{})

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
				timespan = int(mymath.MinMax(tsValue, 15, 1825))
			} else if err != nil {
				logger.Error().Err(err).Str("ts", tsParam).Msg("Failed to interpret timespan (ts) param")
			}
			logger.Info().Int("timespan", timespan).Msg("")
		}

		err := loadTickerDetails(ctx, symbol, timespan)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Add this ticker to recents list
		recents, err := addTickerToRecents(session, r, symbol, acronym)
		webdata["recents"] = Recents{*recents}

		renderTemplateDefault(w, r, "view-daily")
	})
}

func viewTickerIntradayHandler() http.HandlerFunc {
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
		ticker, err := getTickerBySymbol(ctx, symbol)
		if err != nil {
			logger.Warn().Err(err).
				Str("symbol", symbol).
				Int64("exchange_id", exchange.ExchangeId).
				Msg("Failed to find existing ticker")
			http.NotFound(w, r)
			return
		}

		updated, err := ticker.updateTickerIntradays(ctx, intradate)
		if err != nil {
			*messages = append(*messages, Message{fmt.Sprintf("Failed to update ticker intraday data for %s for %s", ticker.TickerSymbol, intradate), "danger"})
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Str("intradate", intradate).
				Msg("Failed to update intrday for ticker")
		}
		if updated {
			*messages = append(*messages, Message{fmt.Sprintf("Ticker intraday data updated for %s for %s", ticker.TickerSymbol, intradate), "success"})
		}

		daily, err := getTickerDailyMostRecent(db, ticker.TickerId)
		if err != nil {
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Msg("Failed to load most recent daily price for ticker")
			http.NotFound(w, r)
			return
		}
		lastTickerDailyMove, err := getLastTickerDailyMove(db, ticker.TickerId)
		if err != nil {
			lastTickerDailyMove = "unknown"
		}

		// load up ticker intradays for date selected
		ticker_intradays, err := ticker.LoadTickerIntraday(db, intradate)
		if err != nil {
			*messages = append(*messages, Message{fmt.Sprintf("No ticker intraday data found for %s", ticker.TickerSymbol), "warning"})
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("tickerId", ticker.TickerId).
				Str("intradate", intradate).
				Msg("Failed to load intraday prices for ticker")
			http.NotFound(w, r)
			return
		}
		if len(ticker_intradays) < 20 {
			*messages = append(*messages, Message{fmt.Sprintf("No ticker intraday data found for %s", ticker.TickerSymbol), "warning"})
			ticker_intradays = []TickerIntraday{}
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

		var lineChartHTML = chartHandlerTickerIntradayLine(ctx, ticker, exchange, ticker_intradays, webwatches, intradate)

		recents, err := addTickerToRecents(session, r, ticker.TickerSymbol, exchange.ExchangeAcronym)

		priorBusinessDay, _ := mytime.PriorBusinessDayStr(intradate + " 21:05:00")
		nextBusinessDay, _ := mytime.NextBusinessDayStr(intradate + " 13:55:00")
		logger.Info().Str("prior", priorBusinessDay).Str("next", nextBusinessDay).Msg("")

		webdata["recents"] = Recents{*recents}
		webdata["ticker"] = ticker
		webdata["exchange"] = exchange
		webdata["ticker_daily"] = daily
		webdata["last_ticker_daily_move"] = lastTickerDailyMove
		webdata["intradate"] = intradate
		webdata["priorBusinessDate"] = priorBusinessDay[0:10]
		webdata["nextBusinessDate"] = nextBusinessDay[0:10]
		webdata["ticker_intradays"] = TickerIntradays{ticker_intradays}
		webdata["watches"] = webwatches
		webdata["lineChart"] = lineChartHTML
		renderTemplateDefault(w, r, "view-intraday")
	})
}
