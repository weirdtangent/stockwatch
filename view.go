package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func viewTickerDailyHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		watcher := checkAuthState(w, r, deps)
		webdata := deps.webdata
		sublog := deps.logger

		params := mux.Vars(r)
		symbol := params["symbol"]
		article := r.FormValue("article")

		// this loads TONS of stuff into webdata
		ticker, err := loadTickerDetails(deps, symbol, 180)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to load ticker details for viewing")
			renderTemplate(w, r, deps, "desktop")
			return
		}

		// Add this ticker to recents list
		watcherRecents, err := addToWatcherRecents(deps, watcher, ticker)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to add ticker to recents list")
		}
		webdata["WatcherRecents"] = watcherRecents

		if article != "" {
			webdata["autoopen_article_encid"] = article
		}

		renderTemplate(w, r, deps, "view-daily")
	})
}
