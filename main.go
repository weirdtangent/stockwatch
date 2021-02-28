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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/weirdtangent/myaws"
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
	clientSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch_google_oauth", "client_secret")
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
	oAuthStateStr, err := myaws.AWSGetSecretKV(awssess, "stockwatch_oauth_state", "oauth_state")
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
	//router.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	router.HandleFunc("/login", googleLoginHandler(oAuthConfig, *oAuthStateStr))
	router.HandleFunc("/callback", googleCallbackHandler(oAuthConfig, *oAuthStateStr))
	router.HandleFunc("/tokensignin", googleTokenSigninHandler(awssess, clientId))
	router.HandleFunc("/view/{symbol}/{acronym}", viewDailyHandler(awssess, db))
	router.HandleFunc("/view/{symbol}/{acronym}/{intradate}", viewIntradayHandler(awssess, db))
	router.HandleFunc("/search/{type}", searchHandler(awssess, db))
	router.HandleFunc("/update/{action}", updateHandler(awssess, db))
	router.HandleFunc("/update/{action}/{symbol}", updateHandler(awssess, db))
	router.HandleFunc("/", homeHandler())

	// middleware chain
	chainedMux1 := withSession(store, router)
	chainedMux2 := withLogging(chainedMux1)

	// starup or die
	server := &http.Server{
		Handler:      chainedMux2,
		Addr:         ":3001",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	if err = server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).
			Msg("Stopped serving requests")
	}
}
