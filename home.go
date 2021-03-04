package main

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/securecookie"
	"github.com/jmoiron/sqlx"
)

func homeHandler(awssess *session.Session, db *sqlx.DB, sc *securecookie.SecureCookie, tmplname string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := make(map[string]interface{})
		messages := make([]Message, 0)
		if tmplname == "home" || tmplname == "terms" || tmplname == "privacy" {
			webdata["hideRecents"] = true
		}
		if tmplname == "home" {
			webdata["allowLogin"] = true
		}

		// the opposite of normal, for authenticated visits we redirect if they were on "home"
		if ok := checkAuthState(w, r, db, sc, webdata); ok && tmplname == "home" {
			http.Redirect(w, r, "/desktop", 302)
		} else {
			webdata["messages"] = Messages{messages}
			renderTemplateDefault(w, r, tmplname, webdata)
		}
		return
	})
}
