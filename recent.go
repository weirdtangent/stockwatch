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

	// if this symbol/exchange is already on the list, remove it so we can add it to the front
	for i, viewed := range recents {
		if viewed == symbol {
			recents = append(recents[:i], recents[i+1:]...)
			break
		}
	}

	// keep only the 4 most recent
	if len(recents) >= 5 {
		recents = recents[:4]
	}
	// prepend latest symbol to front of recents slice
	recents = append([]string{symbol}, recents...)

	session.Values["recents"] = recents

	return &recents, nil
}
