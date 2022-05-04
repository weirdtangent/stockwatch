package main

import (
	"net/http"
)

func homeHandler(deps *Dependencies, tmplname string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata

		checkAuthState(w, r, deps)

		params := r.URL.Query()
		signoutParam := params.Get("signout")
		if signoutParam == "1" {
			deleteWIDCookie(w, r, deps)
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		}

		nextParam := params.Get("next")

		if tmplname == "home" || tmplname == "terms" || tmplname == "privacy" {
			webdata["hideRecents"] = true
		}
		if tmplname == "about" {
			webdata["about"], webdata["commits"], _ = getGithubCommits(deps)
		}
		if len(nextParam) > 0 {
			webdata["next"] = nextParam
		}
		webdata["loggedout"] = true

		renderTemplateDefault(w, r, deps, tmplname)
	})
}
