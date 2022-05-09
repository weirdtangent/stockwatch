package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func viewTickerDailyHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata
		watcher := checkAuthState(w, r, deps)

		params := mux.Vars(r)
		symbol := params["symbol"]

		sublog := deps.logger.With().Str("watcher", watcher.EId).Str("symbol", symbol).Logger()

		tickerQuote, err := getTickerQuote(deps, sublog, watcher, symbol)
		if err != nil {
			sublog.Error().Err(err).Msg("getTickerQuote failed, redirecting to /desktop")
			deps.messages = append(deps.messages, Message{"Sorry, that ticker symbol could not be found", "error"})
			renderTemplate(w, r, deps, sublog, "desktop")
			return
		}
		webdata["TickerQuote"] = tickerQuote

		tickerDetails, err := getTickerDetails(deps, sublog, watcher, symbol)
		if err != nil {
			deps.messages = append(deps.messages, Message{"Sorry, that ticker symbol could not be found", "error"})
			renderTemplate(w, r, deps, sublog, "desktop")
			return
		}
		webdata["TickerDetails"] = tickerDetails

		recents, err := addTickerToWatcherRecents(deps, sublog, watcher, tickerQuote.Ticker)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to add webticker to recents")
		}
		webdata["Recents"] = recents

		renderTemplate(w, r, deps, sublog, "view-daily")
	})
}

func viewTickerArticleHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata
		watcher := checkAuthState(w, r, deps)

		params := mux.Vars(r)
		symbol := params["symbol"]
		articleEId := params["articleEId"]

		sublog := deps.logger.With().Str("watcher", watcher.EId).Str("symbol", symbol).Logger()

		tickerQuote, err := getTickerQuote(deps, sublog, watcher, symbol)
		if err != nil {
			sublog.Error().Err(err).Msg("getTickerQuote failed, redirecting to /desktop")
			deps.messages = append(deps.messages, Message{"Sorry, that ticker symbol could not be found", "error"})
			renderTemplate(w, r, deps, sublog, "desktop")
			return
		}
		webdata["TickerQuote"] = tickerQuote

		tickerDetails, err := getTickerDetails(deps, sublog, watcher, symbol)
		if err != nil {
			deps.messages = append(deps.messages, Message{"Sorry, that ticker symbol could not be found", "error"})
			renderTemplate(w, r, deps, sublog, "desktop")
			return
		}
		webdata["TickerDetails"] = tickerDetails

		recents, err := addTickerToWatcherRecents(deps, sublog, watcher, tickerQuote.Ticker)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to add webticker to recents")
		}
		webdata["Recents"] = recents

		webdata["autoopen_article_encid"] = articleEId

		renderTemplate(w, r, deps, sublog, "view-daily")
	})
}
