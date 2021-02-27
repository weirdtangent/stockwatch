package main

import (
	"net/http"

	"github.com/gorilla/sessions"
)

type Recents struct {
	ViewPairs []ViewPair
}

func getRecents(session *sessions.Session, r *http.Request) (*[]ViewPair, error) {
	// get current list (if any) from session
	recents := session.Values["view_recents"].([]ViewPair)

	return &recents, nil
}

func addTickerToRecents(session *sessions.Session, r *http.Request, symbol string, acronym string) (*[]ViewPair, error) {
	// get current list (if any) from session
	recents := session.Values["view_recents"].([]ViewPair)

	this_view := ViewPair{symbol, acronym}

	// if this symbol/exchange is already on their list just bomb out
	for _, viewed := range recents {
		if viewed == this_view {
			return &recents, nil
		}
	}

	// if they have 5 (or more, somehow), slice it down to just the last 4
	if len(recents) >= 5 {
		recents = recents[len(recents)-4:]
	}
	// now append this new one to the end
	recents = append(recents, this_view)

	// write it to the session
	session.Values["view_recents"] = recents

	return &recents, nil
}
