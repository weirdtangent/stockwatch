package main

import (
	"net/http"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
  recents, _ := getRecents(r)
	renderTemplateDefault(w, r, "home", &DefaultView{*recents})
}
