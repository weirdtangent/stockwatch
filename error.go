package main

import (
	"net/http"
)

func errorHandler(errorMsg string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data = Message{Config: ConfigData{}, MessageText: errorMsg}
		renderTemplateMessages(w, r, "error", &data)
	})
}
