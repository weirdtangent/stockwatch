package main

import (
	"net/http"
)

func homeHandler(tmplname string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		webdata := ctx.Value("webdata").(map[string]interface{})

		params := r.URL.Query()
		nextParam := params.Get("next")

		if tmplname == "home" || tmplname == "terms" || tmplname == "privacy" {
			webdata["hideRecents"] = true
		}
		if tmplname == "home" {
			webdata["allowLogin"] = true
		}
		if len(nextParam) > 0 {
			webdata["next"] = nextParam
		}

		// the opposite of normal, for authenticated visits we redirect if they were on "home"
		if ok := checkAuthState(w, r); ok && tmplname == "home" {
			http.Redirect(w, r, "/desktop", 302)
		} else {
			renderTemplateDefault(w, r, tmplname)
		}
		return
	})
}
