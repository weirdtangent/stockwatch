package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/amazon"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/twitter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/savaki/dynastore"

	"github.com/weirdtangent/myaws"
)

type Dependencies struct {
	awssess      *session.Session
	db           *sqlx.DB
	logger       *zerolog.Logger
	secureCookie *securecookie.SecureCookie
	cookieStore  *dynastore.Store
	redisPool    *redis.Pool
	secrets      map[string]*string
	session      *sessions.Session
	config       map[string]interface{}
	webdata      map[string]interface{}
	request_id   string
	nonce        string
}

func setupLogging() *Dependencies {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	// alter the caller() return to only include the last directory
	zerolog.CallerMarshalFunc = func(file string, line int) string {
		parts := strings.Split(file, "/")
		if len(parts) > 1 {
			return strings.Join(parts[len(parts)-2:], "/") + ":" + strconv.Itoa(line)
		}
		return file + ":" + strconv.Itoa(line)
	}
	pgmPath := strings.Split(os.Args[0], `/`)
	logTag := "stockwatch"
	if len(pgmPath) > 1 {
		logTag = pgmPath[len(pgmPath)-1]
	}
	if debugging {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)

	}
	newlog := log.With().Str("@tag", logTag).Caller().Logger()

	deps := Dependencies{
		logger:  &newlog,
		secrets: make(map[string]*string),
	}

	return &deps
}

func getSecrets(deps *Dependencies) {
	sublog := deps.logger
	awssess := deps.awssess
	secrets := deps.secrets

	secretValues, err := myaws.AWSGetSecret(awssess, "stockwatch")
	if err != nil {
		sublog.Fatal().Err(err)
	}

	for key := range secretValues {
		value := secretValues[key]
		secrets[key] = &value
	}
}

func setupSessionsStore(deps *Dependencies) {
	awssess := deps.awssess
	sublog := deps.logger

	// grab config ---------------------------------------------------------------
	awsConfig, err := myaws.AWSConfig("us-east-1")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to find us-east-1 configuration")
	}

	// connect to Dynamo
	ddb, err := myaws.DDBConnect(awssess)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to connect to DDB")
	}

	// Cookie setup for sessionID ------------------------------------------------
	cookieAuthKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "cookie_auth_key")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}

	cookieEncryptionKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "cookie_encryption_key")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}

	var hashKey = []byte(*cookieAuthKey)
	var blockKey = []byte(*cookieEncryptionKey)
	var secureCookie = securecookie.New(hashKey, blockKey)

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
		sublog.Fatal().Err(err).Msg("failed to setup session management")
	}

	deps.secureCookie = secureCookie
	deps.cookieStore = store
}

func setupOAuth(deps *Dependencies) {
	sublog := deps.logger
	awssess := deps.awssess
	cookieStore := deps.cookieStore
	secrets := deps.secrets

	googleOAuthClientId, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "google_oauth_client_id")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["google_oauth_client_id"] = googleOAuthClientId

	googleOAuthSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "google_oauth_client_secret")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["google_oauth_secret"] = googleOAuthSecret

	twitterApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "twitter_api_key")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	twitterApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "twitter_api_secret")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}

	githubApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "github_api_key")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	githubApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "github_api_secret")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}

	amazonApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "amazon_api_key")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	amazonApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "amazon_api_secret")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}

	facebookApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "facebook_api_key")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}
	facebookApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "facebook_api_secret")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to retrieve secret")
	}

	goth.UseProviders(
		amazon.New(*amazonApiKey, *amazonApiSecret, "https://stockwatch.graystorm.com/auth/amazon/callback"),
		facebook.New(*facebookApiKey, *facebookApiSecret, "https://stockwatch.graystorm.com/auth/facebook/callback", "email"),
		github.New(*githubApiKey, *githubApiSecret, "https://stockwatch.graystorm.com/auth/github/callback"),
		google.New(*googleOAuthClientId, *googleOAuthSecret, "https://stockwatch.graystorm.com/auth/google/callback", "openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"),
		twitter.New(*twitterApiKey, *twitterApiSecret, "https://stockwatch.graystorm.com/auth/twitter/callback"),
	)

	gothic.Store = cookieStore
}

func startHTTPServer(deps *Dependencies) {
	sublog := deps.logger

	// setup middleware chain
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	router.PathPrefix("/favicon.ico").Handler(http.FileServer(http.Dir("static/images")))

	//router.HandleFunc("/tokensignin", signinHandler()).Methods("POST")
	router.HandleFunc("/auth/{provider}", authLoginHandler(deps)).Methods("GET")
	router.HandleFunc("/auth/{provider}/callback", authCallbackHandler(deps)).Methods("GET")
	router.HandleFunc("/signout/", signoutHandler(deps)).Methods("GET")
	router.HandleFunc("/signout/{provider}", signoutHandler(deps)).Methods("GET")
	router.HandleFunc("/logout/", signoutHandler(deps)).Methods("GET")
	router.HandleFunc("/logout/{provider}", signoutHandler(deps)).Methods("GET")

	router.HandleFunc("/ping", pingHandler()).Methods("GET")
	router.HandleFunc("/internal/cspviolations", JSONReportHandler(deps)).Methods("GET")
	router.HandleFunc("/api/v1/{endpoint}", apiV1Handler(deps)).Methods("GET")
	router.Handle("/metrics", promhttp.Handler())

	router.HandleFunc("/profile/{status}", profileHandler(deps)).Methods("GET")
	router.HandleFunc("/desktop", desktopHandler(deps)).Methods("GET")
	router.HandleFunc("/view/{symbol}", viewTickerDailyHandler(deps)).Methods("GET")
	router.HandleFunc("/{action:bought|sold}/{symbol}/{acronym}", transactionHandler(deps)).Methods("POST")
	router.HandleFunc("/search/{type}", searchHandler(deps)).Methods("POST")
	router.HandleFunc("/about", homeHandler(deps, "about")).Methods("GET")
	router.HandleFunc("/terms", homeHandler(deps, "terms")).Methods("GET")
	router.HandleFunc("/privacy", homeHandler(deps, "privacy")).Methods("GET")

	router.HandleFunc("/", homeHandler(deps, "home")).Methods("GET")

	// middleware chain
	chainedMux1 := withSession(router, deps) // deepest level, last to run
	chainedMux2 := withExtraHeader(chainedMux1, deps)
	chainedMux3 := withLogging(chainedMux2, deps)
	chainedMux4 := withContext(chainedMux3, deps) // outer level, first to run

	// starting up web service ---------------------------------------------------
	sublog.Info().Int("port", httpPort).Msg("started serving requests")

	// starup or die
	server := &http.Server{
		Handler:      chainedMux4,
		Addr:         ":" + strconv.Itoa(httpPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		sublog.Fatal().Err(err).Msg("ended abnormally")
	} else {
		sublog.Info().Msg("stopped serving requests")
	}
}
