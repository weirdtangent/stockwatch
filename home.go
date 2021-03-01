package main

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/securecookie"
	"github.com/jmoiron/sqlx"
)

func homeHandler(awssess *session.Session, db *sqlx.DB, sc *securecookie.SecureCookie) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := make(map[string]interface{})
		messages := make([]Message, 0)
		//session := getSession(r)

		// the opposite of normal, for authenticated visits we redirect
		if ok := checkAuthState(w, r, db, sc, webdata); ok {
			http.Redirect(w, r, "/desktop", 302)
		} else {
			webdata["messages"] = messages
			renderTemplateDefault(w, r, "home", webdata)
		}
		return
	})
}
