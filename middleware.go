package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/rs/zerolog/log"
	"github.com/savaki/dynastore"
)

// AddHeaders middleware ------------------------------------------------------

type AddHeader struct {
	handler http.Handler
}

func (ah *AddHeader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	global_nonce = RandStringBytesMask(32)

	header := w.Header()
	csp := []string{
		"default-src 'self'",
		"connect-src 'self' accounts.google.com",
		"style-src 'self' fonts.googleapis.com accounts.google.com 'unsafe-inline'",
		"script-src 'self' apis.google.com accounts.google.com 'nonce-" + global_nonce + "'",
		"font-src 'self' fonts.gstatic.com",
		"frame-src 'self' accounts.google.com",
		"report-uri /internal/cspviolations",
		"report-to default",
	}
	header.Set("Content-Security-Policy", strings.Join(csp, "; "))

	reportTo := `{"group":"default","max-age":1800,"endpoints":[{"url":"https://stockwatch.graystorm.com/internal/cspviolations"}],"include_subdomains":true}`
	header.Set("Report-To", reportTo)

	ah.handler.ServeHTTP(w, r)
}
func withAddHeader(h http.Handler) *AddHeader {
	return &AddHeader{h}
}

// Logging middleware ---------------------------------------------------------

type Logger struct {
	handler http.Handler
}

func (l *Logger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()

	l.handler.ServeHTTP(w, r)

	if r.URL.String() != "/ping" {
		log.Info().
			Stringer("url", r.URL).
			Int("status_code", 200).
			Str("method", r.Method).
			Int64("response_time", time.Since(t).Nanoseconds()).
			Msg("")
	}
}
func withLogging(h http.Handler) *Logger {
	return &Logger{h}
}

// Session management middleware ----------------------------------------------

type Session struct {
	store   *dynastore.Store
	handler http.Handler
}

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := s.store.Get(r, "SID")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get/create session")
	}
	if session.IsNew {
		session.Values["view_recents"] = []ViewPair{}
		session.Values["theme"] = "light"
		err := session.Save(r, w)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to save session")
		}
	}
	r = r.Clone(context.WithValue(r.Context(), "ddbs", session))

	defer session.Save(r, w)

	s.handler.ServeHTTP(w, r)
}
func withSession(store *dynastore.Store, h http.Handler) *Session {
	return &Session{store, h}
}

func getSession(r *http.Request) *sessions.Session {
	session := r.Context().Value("ddbs").(*sessions.Session)
	if session == nil {
		log.Fatal().Err(errFailedToGetSessionFromContext).Msg("")
	}
	return session
}
