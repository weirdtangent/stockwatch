package main

import (
	"net/http"
)

func homeHandler(tmplname string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := checkAuthState(w, r)

		webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})
		params := r.URL.Query()

		signoutParam := params.Get("signout")
		if signoutParam == "1" {
			deleteWIDCookie(w, r)
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		}

		nextParam := params.Get("next")

		if tmplname == "home" || tmplname == "terms" || tmplname == "privacy" {
			webdata["hideRecents"] = true
		}
		if tmplname == "about" {
			webdata["about-contents_template"], webdata["commits"], _ = getGithubCommits(ctx)
		}
		if len(nextParam) > 0 {
			webdata["next"] = nextParam
		}
		webdata["loggedout"] = true

		renderTemplateDefault(w, r, tmplname)
	})
}
