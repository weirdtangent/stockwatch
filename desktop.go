package main

import (
	"net/http"
)

func desktopHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		watcher := checkAuthState(w, r, deps)
		webdata := deps.webdata

		movers := getMovers(deps)
		articles := getRecentArticles(deps)
		recents := getWatcherRecents(deps, watcher)
		recentPlus := getRecentsPlusInfo(deps, recents)
		_, lastCheckedSince, updatingNewsNow := getLastDoneInfo(deps, "financial_news", "stockwatch")

		webdata["Movers"] = movers
		webdata["Articles"] = articles
		webdata["Recents"] = recents
		webdata["RecentPlus"] = recentPlus
		webdata["LastCheckedSince"] = lastCheckedSince
		webdata["UpdatingNewsNow"] = updatingNewsNow

		webdata["Announcement"] = []string{
			"2022-04-22 Moving things around alot, especially on the desktop. Trying to find what I like, but email me if you have ideas!",
		}

		renderTemplate(w, r, deps, "desktop")
	})
}
