package main

import (
	//"fmt"
	//"encoding/gob"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/savaki/dynastore"

	"github.com/weirdtangent/myaws"
)

func main() {
	// setup logging -------------------------------------------------------------
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	// alter the caller() return to only include the last directory
	zerolog.CallerMarshalFunc = func(file string, line int) string {
		parts := strings.Split(file, "/")
		if len(parts) > 1 {
			return strings.Join(parts[len(parts)-2:], "/") + ":" + strconv.Itoa(line)
		}
		return file + ":" + strconv.Itoa(line)
	}
	log.Logger = log.With().Caller().Logger()

	// grab config ---------------------------------------------------------------
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
	_, err = db.Exec("SET NAMES utf8mb4 COLLATE utf8mb4_general_ci")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to switch RDS to UTF8")
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

	// Cookie setup for sessionID ------------------------------------------------
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
	//gob.RegisterName("ViewPair", []ViewPair{})

	// Initialize session manager and configure the session lifetime -------------
	store, err := dynastore.New(
		dynastore.AWSConfig(awsConfig),
		dynastore.DynamoDB(ddb),
		dynastore.TableName("stockwatch-session"),
		dynastore.Secure(),
		dynastore.HTTPOnly(),
		dynastore.Domain("stockwatch.graystorm.com"),
		dynastore.Path("/"),
		dynastore.MaxAge(24*60*60),
		dynastore.Codecs(secureCookie),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to setup session management")
	}

	// starting up web service ---------------------------------------------------
	log.Info().Int("port", 3001).Msg("Started serving requests")

	// setup middleware chain
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	router.PathPrefix("/favicon.ico").Handler(http.FileServer(http.Dir("static/images")))

	router.HandleFunc("/ping", pingHandler()).Methods("GET")
	router.HandleFunc("/internal/cspviolations", JSONReportHandler()).Methods("GET")
	router.HandleFunc("/api/v1/{endpoint}", apiV1Handler()).Methods("GET")

	router.HandleFunc("/login", googleLoginHandler(clientId)).Methods("POST")
	router.HandleFunc("/logout", googleLogoutHandler(clientId)).Methods("GET")
	router.HandleFunc("/desktop", desktopHandler()).Methods("GET")
	router.HandleFunc("/view/{symbol}", viewTickerDailyHandler()).Methods("GET")
	//router.HandleFunc("/view/{symbol}/{acronym}/{intradate}", viewTickerIntradayHandler()).Methods("GET")
	router.HandleFunc("/{action:bought|sold}/{symbol}/{acronym}", transactionHandler()).Methods("POST")
	router.HandleFunc("/search/{type}", searchHandler()).Methods("POST")
	router.HandleFunc("/update/{action}", updateHandler()).Methods("GET")
	router.HandleFunc("/update/{action}/{symbol}", updateHandler()).Methods("GET")
	router.HandleFunc("/terms", homeHandler("terms")).Methods("GET")
	router.HandleFunc("/privacy", homeHandler("privacy")).Methods("GET")

	router.HandleFunc("/", homeHandler("home")).Methods("GET")

	// middleware chain
	chainedMux1 := withSession(store, router) // deepest level, last to run
	chainedMux2 := withAddHeader(chainedMux1)
	chainedMux3 := withAddContext(chainedMux2, awssess, db, secureCookie)
	chainedMux4 := withLogging(chainedMux3) // outer level, first to run

	// starup or die
	server := &http.Server{
		Handler:      chainedMux4,
		Addr:         ":3001",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	if err = server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).
			Msg("Stopped serving requests")
	}
}
