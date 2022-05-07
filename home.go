package main

import (
	"net/http"
)

func homeHandler(deps *Dependencies, tmplname string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuthState(w, r, deps)
		webdata := deps.webdata

		if tmplname == "home" || tmplname == "terms" || tmplname == "privacy" {
			webdata["hideRecents"] = true
		}
		if tmplname == "about" {
			webdata["about"], webdata["commits"], _ = getGithubCommits(deps)
		}

		renderTemplate(w, r, deps, tmplname)
	})
}
