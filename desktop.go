package main

import (
	"net/http"
)

func desktopHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		webdata := ctx.Value("webdata").(map[string]interface{})

		messages := make([]Message, 0)

		if ok := checkAuthState(w, r); ok {
			webdata["messages"] = Messages{messages}
			renderTemplateDefault(w, r, "desktop", webdata)
		} else {
			http.Redirect(w, r, "/", 307)
		}
		return
	})
}
