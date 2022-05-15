package main

import (
	"net/http"
)

func desktopHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		watcher := checkAuthState(w, r, deps, *deps.logger)
		webdata := deps.webdata

		sublog := deps.logger.With().Str("watcher", watcher.EId).Logger()

		movers := getMovers(deps, sublog)
		webdata["Movers"] = movers

		articles := getRecentArticles(deps, sublog)
		webdata["Articles"] = articles

		recents := getWatcherRecents(deps, sublog, watcher)
		tickerQuotes, err := getRecentsQuotes(deps, sublog, watcher, recents)
		if err != nil {
			sublog.Error().Err(err).Msg("getRecentsQuotes failed, redirecting to /desktop")
			deps.messages = append(deps.messages, Message{"Sorry, one or more ticker symbols could not be found", "error"})
			renderTemplate(w, r, deps, sublog, "desktop")
			return
		}
		webdata["TickerQuotes"] = tickerQuotes

		webdata["Announcement"] = []string{
			"2022-04-22 Moving things around alot, especially on the desktop. Trying to find what I like, but email me if you have ideas!",
		}

		renderTemplate(w, r, deps, sublog, "desktop")
	})
}
