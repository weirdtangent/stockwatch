package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/rs/zerolog/log"
	"github.com/savaki/dynastore"
)

// Logging middleware ---------------------------------------------------------

type Logger struct {
	handler http.Handler
}

func (l *Logger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	l.handler.ServeHTTP(w, r)
	log.Info().
		Stringer("url", r.URL).
		Int("status_code", 200).
		Int64("response_time", time.Since(t).Nanoseconds()).
		Msg("")
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
	session, _ := s.store.Get(r, "SID")
	if session.IsNew {
		session.Values["view_recents"] = []ViewPair{}
		session.Values["theme"] = "light"
		session.Save(r, w)
		//if encoded, err := s.store.Codec.Encode("cookie-name", value); err == nil {
		//	cookie := &http.Cookie{
		//		Name:     "cookie-name",
		//		Value:    encoded,
		//		Path:     "/",
		//		Secure:   true,
		//		HttpOnly: true,
		//	}
		//	http.SetCookie(w, cookie)
		//}
	}
	fmt.Printf("%#v\n\n", session)
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
