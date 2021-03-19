package main

import (
	"net/http"
)

func desktopHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//ctx := r.Context()

		if ok := checkAuthState(w, r); ok {
			renderTemplateDefault(w, r, "desktop")
		} else {
			http.Redirect(w, r, "/", 307)
		}
		return
	})
}
