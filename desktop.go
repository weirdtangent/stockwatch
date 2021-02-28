package main

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/securecookie"
	"github.com/jmoiron/sqlx"
	//"github.com/rs/zerolog/log"
)

func desktopHandler(awssess *session.Session, db *sqlx.DB, sc *securecookie.SecureCookie) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := make(map[string]interface{})
		messages := make([]Message, 0)
		//session := getSession(r)
		if ok := checkAuthState(r, db, sc, webdata); ok {
			webdata["messages"] = messages
			renderTemplateDefault(w, r, "desktop", webdata)
		} else {
			http.Redirect(w, r, "/", 401)
		}
		return
	})
}
