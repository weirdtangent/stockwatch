package main

import (
	"net/http"
)

func errorHandler(w http.ResponseWriter, r *http.Request, errorMsg string) {
	var data = Message{errorMsg}
	renderTemplateMessages(w, r, "error", &data)
}
