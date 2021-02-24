package main

import (
	"net/http"

	"github.com/alexedwards/scs"
)

func homeHandler(smgr *scs.SessionManager) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recents, _ := getRecents(smgr, r)
		renderTemplateDefault(w, r, "home", &DefaultView{Config: ConfigData{}, Recents: *recents})
	})
}
