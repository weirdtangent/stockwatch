package main

import (
	//"fmt"
	"encoding/gob"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/savaki/dynastore"

	"github.com/weirdtangent/myaws"
)

var (
	global_nonce string
)

func main() {
	// setup logging
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.With().Caller().Logger()

	// grab config
	awsConfig, err := myaws.AWSConfig("us-east-1")

	// connect to AWS
	awssess, err := myaws.AWSConnect("us-east-1", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to AWS")
	}

	// connect to Aurora
	db, err := myaws.DBConnect(awssess, "stockwatch_rds", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to RDS")
	}

	// connect to Dynamo
	ddb, err := myaws.DDBConnect(awssess)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to DDB")
	}

	// config Google OAuth
	clientId, err := myaws.AWSGetSecretKV(awssess, "stockwatch_google_oauth", "client_id")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to retrieve secret")
	}

	// Cookie setup for sessionID
	cookieAuthKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch_cookie", "auth_key")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to retrieve secret")
	}
	cookieEncryptionKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch_cookie", "encryption_key")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to retrieve secret")
	}
	var hashKey = []byte(*cookieAuthKey)
	var blockKey = []byte(*cookieEncryptionKey)
	var secureCookie = securecookie.New(hashKey, blockKey)
	gob.RegisterName("ViewPair", []ViewPair{})

	// Initialize a new session manager and configure the session lifetime.
	store, err := dynastore.New(
		dynastore.AWSConfig(awsConfig),
		dynastore.DynamoDB(ddb),
		dynastore.TableName("stockwatch-session"),
		dynastore.Secure(),
		dynastore.HTTPOnly(),
		dynastore.Domain("stockwatch.graystorm.com"),
		dynastore.Path("/"),
		dynastore.MaxAge(900),
		dynastore.Codecs(secureCookie),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to setup session management")
	}

	// starting up
	log.Info().Int("port", 3001).Msg("Started serving requests")

	// setup middleware chain
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	router.PathPrefix("/favicon.ico").Handler(http.FileServer(http.Dir("static/images")))
	//router.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	router.HandleFunc("/ping", pingHandler())
	router.HandleFunc("/internal/cspviolations", JSONReportHandler(awssess))
	router.HandleFunc("/login", googleLoginHandler(awssess, db, secureCookie, clientId))
	router.HandleFunc("/logout", googleLogoutHandler(awssess, db, secureCookie, clientId))
	router.HandleFunc("/desktop", desktopHandler(awssess, db, secureCookie))
	router.HandleFunc("/view/{symbol}/{acronym}", viewDailyHandler(awssess, db, secureCookie))
	router.HandleFunc("/view/{symbol}/{acronym}/{intradate}", viewIntradayHandler(awssess, db, secureCookie))
	router.HandleFunc("/search/{type}", searchHandler(awssess, db))
	router.HandleFunc("/update/{action}", updateHandler(awssess, db))
	router.HandleFunc("/update/{action}/{symbol}", updateHandler(awssess, db))
	router.HandleFunc("/", homeHandler(awssess, db, secureCookie))

	// middleware chain
	chainedMux1 := withSession(store, router)
	chainedMux2 := withAddHeader(chainedMux1)
	chainedMux3 := withLogging(chainedMux2)

	// starup or die
	server := &http.Server{
		Handler:      chainedMux3,
		Addr:         ":3001",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	if err = server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).
			Msg("Stopped serving requests")
	}
}
