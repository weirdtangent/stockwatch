package main

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs"
	"github.com/alexedwards/scs/mysqlstore"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"graystorm.com/myaws"
)

var sessionManager *scs.SessionManager
var aws_session *session.Session
var db_session *sqlx.DB
var verbose = true

func init() {
	// setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.With().Caller().Logger()

	// connect to AWS
	var err error
	aws_session, err = myaws.AWSConnect("us-east-1", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to AWS")
	}

	// connect to Aurora
	db_session, err = myaws.DBConnect(aws_session, "stockwatch_rds", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect the RDS")
	}

	// Initialize a new session manager and configure the session lifetime.
	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Store = mysqlstore.New(db_session.DB)
	sessionManager.Cookie.Domain = "stockwatch.graystorm.com"
}

func main() {
	log.Info().Int("port", 3001).Msg("Started serving requests")
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	mux.HandleFunc("/view/", viewHandler)
	mux.HandleFunc("/search/", searchHandler)
	mux.HandleFunc("/update/", updateHandler)
	mux.HandleFunc("/", homeHandler)

	// starup or die
	err := http.ListenAndServe(":3001", sessionManager.LoadAndSave(mux))
	log.Fatal().Err(err).Msg("Stopped serving requests")
}
