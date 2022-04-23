package main

import (
	"net/http"
)

func desktopHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		//	messages := ctx.Value(ContextKey("messages")).(*[]Message)
		webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})

		if ok := checkAuthState(w, r); !ok {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		movers, _ := getMovers(ctx)
		webdata["movers"] = movers

		articles, _ := getArticlesByKeyword(ctx, "")
		webdata["articles"] = articles

		recentPlus, _ := getRecentsPlusInfo(ctx, r)
		webdata["recentplus"] = recentPlus

		webdata["announcement"] = []string{
			"Moving things around alot, especially on the desktop. Trying to find what I like, but email me if you have ideas!",
		}

		renderTemplateDefault(w, r, "desktop")
	})
}
