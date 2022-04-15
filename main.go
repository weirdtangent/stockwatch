package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/savaki/dynastore"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/twitter"

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
	if err != nil {
		log.Fatal().Err(err).Msg("failed to find us-east-1 configuration")
	}

	// connect to AWS
	awssess, err := myaws.AWSConnect("us-east-1", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to AWS")
	}

	// connect to MySQL
	db := myaws.DBMustConnect(awssess, "stockwatch", "stockwatch")

	_, err = db.Exec("SET NAMES utf8mb4 COLLATE utf8mb4_general_ci")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to switch RDS to UTF8")
	}

	// connect to Dynamo
	ddb, err := myaws.DDBConnect(awssess)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to DDB")
	}

	var secrets = make(map[string]string)

	// Cookie setup for sessionID ------------------------------------------------
	cookieAuthKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "cookie_auth_key")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["cookie_auth_key"] = *cookieAuthKey

	cookieEncryptionKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "cookie_encryption_key")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["cookie_encryption_key"] = *cookieEncryptionKey

	var hashKey = []byte(*cookieAuthKey)
	var blockKey = []byte(*cookieEncryptionKey)
	var secureCookie = securecookie.New(hashKey, blockKey)

	// Cache all other secrets into global map -----------------------------------

	// get yahoofinance api access key and host
	yf_api_access_key, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "yahoofinance_rapidapi_key")
	if err != nil {
		log.Fatal().Err(err).
			Msg("failed to get Yahoo Finance API key")
	}
	secrets["yahoofinance_rapidapi_key"] = *yf_api_access_key

	yf_api_access_host, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "yahoofinance_rapidapi_host")
	if err != nil {
		log.Fatal().Err(err).
			Msg("failed to get Yahoo Finance API key")
	}
	secrets["yahoofinance_rapidapi_host"] = *yf_api_access_host

	// get morningstar api access key and host
	ms_api_access_key, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "morningstar_rapidapi_key")
	if err != nil {
		log.Fatal().Err(err).
			Msg("failed to get Morningstar API key")
	}
	secrets["morningstar_rapidapi_key"] = *ms_api_access_key

	ms_api_access_host, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "morningstar_rapidapi_host")
	if err != nil {
		log.Fatal().Err(err).
			Msg("failed to get Morningstar API key")
	}
	secrets["morningstar_rapidapi_host"] = *ms_api_access_host

	// get bloomberg api access key and host
	bb_api_access_key, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "bloomberg_rapidapi_key")
	if err != nil {
		log.Fatal().Err(err).
			Msg("failed to get Bloomberg API key")
	}
	secrets["bloomberg_rapidapi_key"] = *bb_api_access_key

	bb_api_access_host, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "bloomberg_rapidapi_host")
	if err != nil {
		log.Fatal().Err(err).
			Msg("failed to get Bloomberg API key")
	}
	secrets["bloomberg_rapidapi_host"] = *bb_api_access_host

	// config Google OAuth
	googleOAuthClientId, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "google_oauth_client_id")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["google_oauth_client_id"] = *googleOAuthClientId

	googleOAuthSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "google_oauth_client_secret")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["google_oauth_secret"] = *googleOAuthSecret

	// github OAuth key
	githubOAuthKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "github_oauth_key")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["github_oauth_key"] = *githubOAuthKey

	// google svc account
	google_svc_acct, err := myaws.AWSGetSecretValue(awssess, "stockwatch_google_svc_acct")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["stockwatch_google_svc_acct"] = *google_svc_acct

	// stockwatch next url encryption key
	next_url_key, err := myaws.AWSGetSecretValue(awssess, "stockwatch_next_url_key")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["next_url_key"] = *next_url_key

	// Initialize session manager and configure the session lifetime -------------
	store, err := dynastore.New(
		dynastore.AWSConfig(awsConfig),
		dynastore.DynamoDB(ddb),
		dynastore.TableName("stockwatch-session"),
		dynastore.Secure(),
		dynastore.HTTPOnly(),
		dynastore.Domain("stockwatch.graystorm.com"),
		dynastore.Path("/"),
		dynastore.MaxAge(31*24*60*60),
		dynastore.Codecs(secureCookie),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup session management")
	}

	// auth api setup ---------------------------------------------------------
	twitterApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "twitter_api_key")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["twitter_api_key"] = *twitterApiKey

	twitterApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "twitter_api_secret")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["twitter_api_secret"] = *twitterApiSecret

	githubApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "github_api_key")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["github_api_key"] = *githubApiKey

	githubApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "github_api_secret")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["github_api_secret"] = *githubApiSecret

	goth.UseProviders(
		google.New(*googleOAuthClientId, *googleOAuthSecret, "https://stockwatch.graystorm.com/auth/google/callback", "openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"),
		twitter.New(*twitterApiKey, *twitterApiSecret, "https://stockwatch.graystorm.com/auth/twitter/callback"),
		github.New(*githubApiKey, *githubApiSecret, "https://stockwatch.graystorm.com/auth/github/callback"),
	)
	gothic.Store = store

	// starting up web service ---------------------------------------------------
	log.Info().Int("port", 3001).Msg("Started serving requests")

	// setup middleware chain
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	router.PathPrefix("/favicon.ico").Handler(http.FileServer(http.Dir("static/images")))

	//router.HandleFunc("/tokensignin", signinHandler()).Methods("POST")
	router.HandleFunc("/signout/{provider}", signoutHandler()).Methods("GET")
	router.HandleFunc("/auth/{provider}", authLoginHandler()).Methods("GET")
	router.HandleFunc("/auth/{provider}/callback", authCallbackHandler()).Methods("GET")

	router.HandleFunc("/ping", pingHandler()).Methods("GET")
	router.HandleFunc("/internal/cspviolations", JSONReportHandler()).Methods("GET")
	router.HandleFunc("/api/v1/{endpoint}", apiV1Handler()).Methods("GET")
	router.Handle("/metrics", promhttp.Handler())

	router.HandleFunc("/profile", profileHandler()).Methods("GET")
	router.HandleFunc("/desktop", desktopHandler()).Methods("GET")
	router.HandleFunc("/view/{symbol}", viewTickerDailyHandler()).Methods("GET")
	router.HandleFunc("/{action:bought|sold}/{symbol}/{acronym}", transactionHandler()).Methods("POST")
	router.HandleFunc("/search/{type}", searchHandler()).Methods("POST")
	router.HandleFunc("/update/{action}", updateHandler()).Methods("GET")
	router.HandleFunc("/about", homeHandler("about")).Methods("GET")
	router.HandleFunc("/terms", homeHandler("terms")).Methods("GET")
	router.HandleFunc("/privacy", homeHandler("privacy")).Methods("GET")

	router.HandleFunc("/", homeHandler("home")).Methods("GET")

	// middleware chain
	chainedMux1 := withSession(store, router) // deepest level, last to run
	chainedMux2 := withAddHeader(chainedMux1)
	chainedMux3 := withAddContext(chainedMux2, awssess, db, secureCookie, secrets)
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
