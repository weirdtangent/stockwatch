package main

import (
	"encoding/json"
	"net/http"

	"github.com/alexedwards/scs"
)

func getRecents(sm *scs.SessionManager, r *http.Request) (*[]ViewPair, error) {
	recents := []ViewPair{}

	// get current list (if any) from session
	recents_json := sm.GetBytes(r.Context(), "view_recents")
	json.Unmarshal(recents_json, &recents)

	return &recents, nil
}

func addTickerToRecents(sm *scs.SessionManager, r *http.Request, symbol string, acronym string) (*[]ViewPair, error) {
	var recents []ViewPair
	this_view := ViewPair{symbol, acronym}

	// get current list (if any) from session
	recents_json := sm.GetBytes(r.Context(), "view_recents")
	if len(recents_json) > 0 {
		json.Unmarshal(recents_json, &recents)
	}

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
	recents_json, err := json.Marshal(recents)
	if err == nil {
		sm.Put(r.Context(), "view_recents", recents_json)
	}

	return &recents, err
}
