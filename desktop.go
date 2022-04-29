package main

import (
	"net/http"
)

func desktopHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, watcher := checkAuthState(w, r)

		webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})

		movers, _ := getMovers(ctx)
		articles, _ := getRecentArticles(ctx)
		watcherRecents := getWatcherRecents(ctx, watcher)
		recentPlus, _ := getRecentsPlusInfo(ctx, watcherRecents)

		webdata["movers"] = movers
		webdata["articles"] = articles
		webdata["WatcherRecents"] = watcherRecents
		webdata["recentplus"] = recentPlus
		webdata["announcement"] = []string{
			"2022-04-22 Moving things around alot, especially on the desktop. Trying to find what I like, but email me if you have ideas!",
		}

		renderTemplateDefault(w, r, "desktop")
	})
}
