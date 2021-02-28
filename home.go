package main

import (
	"net/http"

	"github.com/gorilla/securecookie"
)

func homeHandler(sc *securecookie.SecureCookie) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := make(map[string]interface{})
		messages := make([]Message, 0)
		//session := getSession(r)
		if ok := checkAuthState(r, sc, webdata); ok {
			webdata["messages"] = messages
			renderTemplateDefault(w, r, "home", webdata)
		} else {
			http.Redirect(w, r, "/", 401)
		}
		return
	})
}
