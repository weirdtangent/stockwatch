package main

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs"
	"github.com/alexedwards/scs/mysqlstore"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"graystorm.com/myaws"
)

type Logger struct {
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
	smgr := scs.New()
	smgr.Lifetime = 365 * 24 * time.Hour
	smgr.Store = mysqlstore.New(db.DB)
	smgr.Cookie.Domain = "stockwatch.graystorm.com"

	// starting up
	log.Info().Int("port", 3001).Msg("Started serving requests")

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	mux.HandleFunc("/login", googleLoginHandler(oAuthConfig, oAuthStateStr))
	mux.HandleFunc("/callback", googleCallbackHandler(oAuthConfig, oAuthStateStr))
	mux.HandleFunc("/tokensignin", googleTokenSigninHandler(aws, clientId, smgr))
	mux.HandleFunc("/view/", viewHandler(aws, db, smgr))
	mux.HandleFunc("/search/", searchHandler(aws, db))
	mux.HandleFunc("/update/", updateHandler(aws, db))
	mux.HandleFunc("/", homeHandler(smgr))

	//wrappedMux := NewLogger(mux)

	// starup or die
	if err = http.ListenAndServe(":3001", smgr.LoadAndSave(NewLogger(mux))); err != nil {
		log.Fatal().Err(err).
			Msg("Stopped serving requests")
	}
}

//ServeHTTP handles the request by passing it to the real
//handler and logging the request details
func (l *Logger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	l.handler.ServeHTTP(w, r)
	log.Info().
		Stringer("url", r.URL).
		Int("status_code", 200).
		Int64("response_time", time.Since(t).Nanoseconds()).
		Msg("request served")
}

//NewLogger constructs a new Logger middleware handler
func NewLogger(handlerToWrap http.Handler) *Logger {
	return &Logger{handlerToWrap}
}
