package main

import (
	"net/http"

	"github.com/gorilla/sessions"
)

func getRecents(session *sessions.Session, r *http.Request) (*[]string, error) {
	// get current list (if any) from session
	var recents []string

	if session.Values["recents"] != nil {
		recents = session.Values["recents"].([]string)
	}

	return &recents, nil
}

func addTickerToRecents(session *sessions.Session, r *http.Request, symbol string) (*[]string, error) {
	// get current list (if any) from session
	var recents []string

	if session.Values["recents"] != nil {
		recents = session.Values["recents"].([]string)
	}

	// if this symbol/exchange is already on their list just bomb out
	for _, viewed := range recents {
		if viewed == symbol {
			return &recents, nil
		}
	}

	if len(recents) >= 5 {
		recents = recents[len(recents)-4:]
	}
	recents = append(recents, symbol)

	session.Values["recents"] = recents

	return &recents, nil
}
