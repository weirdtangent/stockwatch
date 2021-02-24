package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/savaki/dynastore"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/weirdtangent/myaws"
)

type Logger struct {
	handler http.Handler
}
type Session struct {
	store   *dynastore.Store
	name    string
	handler http.Handler
}

func main() {
	// setup logging
	//zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.With().Caller().Logger()

	// connect to AWS
	aws, err := myaws.AWSConnect("us-east-1", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to AWS")
	}

	// connect to Aurora
	db, err := myaws.DBConnect(aws, "stockwatch_rds", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect the RDS")
	}

	// config Google OAuth
	clientId, err := myaws.AWSGetSecretKV(aws, "stockwatch_google_oauth", "client_id")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to retrieve secret")
	}
	clientSecret, err := myaws.AWSGetSecretKV(aws, "stockwatch_google_oauth", "client_secret")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to retrieve secret")
	}
	oAuthConfig := &oauth2.Config{
		RedirectURL:  "https://stockwatch.graystorm.com/callback",
		ClientID:     *clientId,
		ClientSecret: *clientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	var oAuthStateStr = "vXrwhPrewsQxJKX6Bg9H86MoEC3PfPwv"

	// Initialize a new session manager and configure the session lifetime.
	store, err := dynastore.New(dynastore.Path("/"), dynastore.HTTPOnly(), dynastore.MaxAge(900))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to setup session management")
	}
	var sessionKey = "session_key"

	// starting up
	log.Info().Int("port", 3001).Msg("Started serving requests")

	// setup middleware chain
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	mux.HandleFunc("/login", googleLoginHandler(oAuthConfig, oAuthStateStr))
	mux.HandleFunc("/callback", googleCallbackHandler(oAuthConfig, oAuthStateStr))
	mux.HandleFunc("/tokensignin", googleTokenSigninHandler(aws, clientId))
	mux.HandleFunc("/view/", viewHandler(aws, db))
	mux.HandleFunc("/search/", searchHandler(aws, db))
	mux.HandleFunc("/update/", updateHandler(aws, db))
	mux.HandleFunc("/", homeHandler())

	// middleware chain
	chainedMux1 := withSession(store, sessionKey, mux)
	chainedMux2 := withLogging(chainedMux1)

	// starup or die
	if err = http.ListenAndServe(":3001", chainedMux2); err != nil {
		log.Fatal().Err(err).
			Msg("Stopped serving requests")
	}
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

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, s.name)
	if session.IsNew {
		session.Values["view_recents"] = []ViewPair{}
		session.Save(r, w)
	}
	defer session.Save(r, w)

	ctx := context.WithValue(r.Context(), "session", session)
	s.handler.ServeHTTP(w, r.WithContext(ctx))
}
func withSession(store *dynastore.Store, name string, h http.Handler) *Session {
	return &Session{store, name, h}
}

func getSession(r *http.Request) *sessions.Session {
	session := r.Context().Value("session").(*sessions.Session)
	if session == nil {
		log.Fatal().Err(errFailedToGetSessionFromContext).Msg("")
	}
	return session
}
