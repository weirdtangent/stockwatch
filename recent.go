package main

import (
	"net/http"

	"github.com/gorilla/sessions"
)

func getRecents(session *sessions.Session, r *http.Request) (*[]string, error) {
	//logger := log.Ctx(r.Context())
	// get current list (if any) from session
	var recents []string

	if session.Values["recents"] != nil {
		recents = session.Values["recents"].([]string)
	}

	return &recents, nil
}

func addTickerToRecents(session *sessions.Session, r *http.Request, symbol string) (*[]string, error) {
	//logger := log.Ctx(r.Context())
	// get current list (if any) from session
	recents := session.Values["recents"].([]string)

	// if this symbol/exchange is already on their list just bomb out
	for _, viewed := range recents {
		if viewed == symbol {
			return &recents, nil
		}
	}

	// if they have 5 (or more, somehow), slice it down to just the last 4
	if len(recents) >= 5 {
		recents = recents[len(recents)-4:]
	}
	// now append this new one to the end
	recents = append(recents, symbol)

	// write it to the session
	session.Values["recents"] = recents

	return &recents, nil
}
