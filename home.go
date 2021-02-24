package main

import (
	"net/http"
)

func homeHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)
		recents, _ := getRecents(session, r)
		renderTemplateDefault(w, r, "home", &DefaultView{Config: ConfigData{}, Recents: *recents})
	})
}
