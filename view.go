package main

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/weirdtangent/mymath"
)

func viewTickerDailyHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		watcher := checkAuthState(w, r, deps)

		// messages := *(deps.messages)
		webdata := deps.webdata
		sublog := deps.logger

		params := mux.Vars(r)
		symbol := params["symbol"]

		timespan := 180
		if tsParam := r.FormValue("ts"); tsParam != "" {
			if tsValue, err := strconv.ParseInt(tsParam, 10, 32); err == nil {
				timespan = int(mymath.MinMax(tsValue, 15, 1825))
			} else if err != nil {
				sublog.Error().Err(err).Str("ts", tsParam).Msg("invalid timespan (ts) param")
			}
			sublog.Info().Int("timespan", timespan).Msg("")
		}

		ticker, err := loadTickerDetails(deps, symbol, timespan)
		if err != nil {
			sublog.Error().Err(err).Msg("Failed to load ticker details for viewing")
			// messages = append(messages, Message{fmt.Sprintf("Sorry, I had trouble loading that stock: %s", err.Error()), "danger"})
			renderTemplateDefault(w, r, deps, "desktop")
			return
		}

		lastCheckedNews, updatingNewsNow := getNewsLastUpdated(deps, ticker)
		webdata["LastCheckedNews"] = lastCheckedNews
		webdata["UpdatingNewsNow"] = updatingNewsNow
		webdata["TickerFavIconCDATA"] = ticker.getFavIconCDATA(deps)

		// Add this ticker to recents list
		watcherRecents, err := addTickerToRecents(deps, watcher, ticker)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to add ticker to recents list")
		}
		webdata["WatcherRecents"] = watcherRecents

		renderTemplateDefault(w, r, deps, "view-daily")
	})
}
