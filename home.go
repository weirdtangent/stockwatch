package main

import (
	"net/http"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, "home")
}
