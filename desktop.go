package main

import (
	"net/http"
)

func desktopHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata
		sublog := deps.logger

		watcher := checkAuthState(w, r, deps)

		movers, err := getMovers(deps)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to get movers")
		}
		articles, _ := getRecentArticles(deps)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to get recent articles")
		}
		watcherRecents := getWatcherRecents(deps, watcher)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to get recents")
		}
		recentPlus, _ := getRecentsPlusInfo(deps, watcherRecents)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to get recent_plus")
		}

		webdata["Movers"] = movers
		webdata["Articles"] = articles
		webdata["WatcherRecents"] = watcherRecents
		webdata["RecentPlus"] = recentPlus
		webdata["Announcement"] = []string{
			"2022-04-22 Moving things around alot, especially on the desktop. Trying to find what I like, but email me if you have ideas!",
		}

		renderTemplateDefault(w, r, deps, "desktop")
	})
}
