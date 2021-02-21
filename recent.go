package main

import (
	"encoding/json"
	"net/http"
)

func addTickerToRecent(r *http.Request, symbol string, acronym string) (*[]ViewPair, error) {
	var recent []ViewPair
	this_view := ViewPair{symbol, acronym}

	// get current list (if any) from session
	recent_json := sessionManager.GetBytes(r.Context(), "view_recent")
	if len(recent_json) > 0 {
		json.Unmarshal(recent_json, &recent)
	}

	// if this symbol/exchange is already on their list just bomb out
	for _, viewed := range recent {
		if viewed == this_view {
			return &recent, nil
		}
	}

	// if they have 5 (or more, somehow), slice it down to just the last 4
	if len(recent) >= 5 {
		recent = recent[len(recent)-4:]
	}
	// now append this new one to the end
	recent = append(recent, this_view)

	// write it to the session
	recent_json, err := json.Marshal(recent)
	if err == nil {
		sessionManager.Put(r.Context(), "view_recent", recent_json)
	}

	return &recent, err
}
