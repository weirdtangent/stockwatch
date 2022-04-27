package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
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

func setupLogging() context.Context {
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
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if debugging {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)

	}
	log := log.With().Str("@tag", logTag).Caller().Logger()
	ctx := log.WithContext(context.Background())

	return ctx
}

func setupOAuth(ctx context.Context, awssess *session.Session, store *dynastore.Store, secrets *map[string]string) {
	googleOAuthClientId, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "google_oauth_client_id")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	(*secrets)["google_oauth_client_id"] = *googleOAuthClientId

	googleOAuthSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "google_oauth_client_secret")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	(*secrets)["google_oauth_secret"] = *googleOAuthSecret

	twitterApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "twitter_api_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	twitterApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "twitter_api_secret")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}

	githubApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "github_api_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	githubApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "github_api_secret")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}

	amazonApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "amazon_api_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	amazonApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "amazon_api_secret")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}

	facebookApiKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "facebook_api_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	facebookApiSecret, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "facebook_api_secret")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}

	goth.UseProviders(
		amazon.New(*amazonApiKey, *amazonApiSecret, "https://stockwatch.graystorm.com/auth/amazon/callback"),
		facebook.New(*facebookApiKey, *facebookApiSecret, "https://stockwatch.graystorm.com/auth/facebook/callback", "email"),
		github.New(*githubApiKey, *githubApiSecret, "https://stockwatch.graystorm.com/auth/github/callback"),
		google.New(*googleOAuthClientId, *googleOAuthSecret, "https://stockwatch.graystorm.com/auth/google/callback", "openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"),
		twitter.New(*twitterApiKey, *twitterApiSecret, "https://stockwatch.graystorm.com/auth/twitter/callback"),
	)

	gothic.Store = store
}

func setupSessionsStore(ctx context.Context, awssess *session.Session) (*securecookie.SecureCookie, *dynastore.Store) {
	// grab config ---------------------------------------------------------------
	awsConfig, err := myaws.AWSConfig("us-east-1")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to find us-east-1 configuration")
	}

	// connect to Dynamo
	ddb, err := myaws.DDBConnect(awssess)
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to connect to DDB")
	}

	// Cookie setup for sessionID ------------------------------------------------
	cookieAuthKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "cookie_auth_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}

	cookieEncryptionKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "cookie_encryption_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
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
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to setup session management")
	}

	return secureCookie, store
}

func getSecrets(ctx context.Context, awssess *session.Session) map[string]string {
	var secrets = make(map[string]string)

	// get yhfinance api access key and host
	yf_api_access_key, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "yhfinance_rapidapi_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).
			Msg("failed to get YHFinance API key")
	}
	secrets["yhfinance_rapidapi_key"] = *yf_api_access_key

	yf_api_access_host, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "yhfinance_rapidapi_host")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).
			Msg("failed to get YHFinance API key")
	}
	secrets["yhfinance_rapidapi_host"] = *yf_api_access_host

	// get msfinance api access key and host
	ms_api_access_key, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "msfinance_rapidapi_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).
			Msg("failed to get Morningstar API key")
	}
	secrets["msfinance_rapidapi_key"] = *ms_api_access_key

	ms_api_access_host, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "msfinance_rapidapi_host")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).
			Msg("failed to get Morningstar API key")
	}
	secrets["msfinance_rapidapi_host"] = *ms_api_access_host

	// get bbfinance api access key and host
	bb_api_access_key, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "bbfinance_rapidapi_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).
			Msg("failed to get bbfinance API key")
	}
	secrets["bbfinance_rapidapi_key"] = *bb_api_access_key

	bb_api_access_host, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "bbfinance_rapidapi_host")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).
			Msg("failed to get bbfinance API key")
	}
	secrets["bbfinance_rapidapi_host"] = *bb_api_access_host

	// google svc account
	google_svc_acct, err := myaws.AWSGetSecretValue(awssess, "stockwatch_google_svc_acct")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["stockwatch_google_svc_acct"] = *google_svc_acct

	// github svc account
	githubOAuthKey, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "github_oauth_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["github_oauth_key"] = *githubOAuthKey

	// stockwatch next url encryption key
	next_url_key, err := myaws.AWSGetSecretValue(awssess, "stockwatch_next_url_key")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["next_url_key"] = *next_url_key

	skip64_watcher, err := myaws.AWSGetSecretKV(awssess, "stockwatch", "skip64_watcher")
	if err != nil || *skip64_watcher == "" {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to retrieve secret")
	}
	secrets["skip64_watcher"] = *skip64_watcher

	return secrets
}

func startHTTPServer(ctx context.Context, awssess *session.Session, db *sqlx.DB, secrets map[string]string, store *dynastore.Store, secureCookie *securecookie.SecureCookie) {
	// setup middleware chain
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	router.PathPrefix("/favicon.ico").Handler(http.FileServer(http.Dir("static/images")))

	//router.HandleFunc("/tokensignin", signinHandler()).Methods("POST")
	router.HandleFunc("/auth/{provider}", authLoginHandler()).Methods("GET")
	router.HandleFunc("/auth/{provider}/callback", authCallbackHandler()).Methods("GET")
	router.HandleFunc("/signout/{provider}", signoutHandler()).Methods("GET")
	router.HandleFunc("/logout/{provider}", signoutHandler()).Methods("GET")

	router.HandleFunc("/ping", pingHandler()).Methods("GET")
	router.HandleFunc("/internal/cspviolations", JSONReportHandler()).Methods("GET")
	router.HandleFunc("/api/v1/{endpoint}", apiV1Handler()).Methods("GET")
	router.Handle("/metrics", promhttp.Handler())

	router.HandleFunc("/profile", profileHandler()).Methods("GET")
	router.HandleFunc("/desktop", desktopHandler()).Methods("GET")
	router.HandleFunc("/view/{symbol}", viewTickerDailyHandler()).Methods("GET")
	router.HandleFunc("/{action:bought|sold}/{symbol}/{acronym}", transactionHandler()).Methods("POST")
	router.HandleFunc("/search/{type}", searchHandler()).Methods("POST")
	router.HandleFunc("/about", homeHandler("about")).Methods("GET")
	router.HandleFunc("/terms", homeHandler("terms")).Methods("GET")
	router.HandleFunc("/privacy", homeHandler("privacy")).Methods("GET")

	router.HandleFunc("/", homeHandler("home")).Methods("GET")

	// middleware chain
	chainedMux1 := withSession(store, router) // deepest level, last to run
	chainedMux2 := withAddHeader(chainedMux1)
	chainedMux3 := withAddContext(chainedMux2, awssess, db, secureCookie, secrets)
	chainedMux4 := withLogging(chainedMux3) // outer level, first to run

	// starting up web service ---------------------------------------------------
	zerolog.Ctx(ctx).Info().Int("port", httpPort).Msg("started serving requests")

	// starup or die
	server := &http.Server{
		Handler:      chainedMux4,
		Addr:         ":" + strconv.Itoa(httpPort),
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("ended abnormally")
	} else {
		zerolog.Ctx(ctx).Info().Msg("stopped serving requests")
	}
}
