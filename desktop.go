package main

import (
	"net/http"
)

func desktopHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		//	messages := ctx.Value("messages").(*[]Message)
		webdata := ctx.Value("webdata").(map[string]interface{})

		if ok := checkAuthState(w, r); ok == false {
			http.Redirect(w, r, "/", 307)
			return
		}

		movers, _ := getMovers(ctx)
		webdata["movers"] = movers

		articles, _ := getArticlesByKeyword(ctx, "")
		webdata["articles"] = articles

		renderTemplateDefault(w, r, "desktop")
	})
}
