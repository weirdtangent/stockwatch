package main

import (
	"net/http"
)

func homeHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)
		recents, _ := getRecents(session, r)
		var messages = make([]Message, 0)

		webdata := make(map[string]interface{})
		webdata["config"] = ConfigData{}
		webdata["recents"] = *recents
		webdata["messages"] = messages
		renderTemplateDefault(w, r, "home", webdata)
	})
}
