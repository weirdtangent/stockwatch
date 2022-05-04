package main

import (
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
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
	awsconfig    *aws.Config
	awssess      *session.Session
	db           *sqlx.DB
	ddb          *dynamodb.DynamoDB
	logger       *zerolog.Logger
	secureCookie *securecookie.SecureCookie
	cookieStore  *dynastore.Store
	redisPool    *redis.Pool
	templates    *template.Template
	secrets      map[string]string
	session      *sessions.Session
	config       map[string]interface{}
	webdata      map[string]interface{}
	request_id   string
	nonce        string
}

func setupLogging(deps *Dependencies) {
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
	}
	newlog := log.With().Str("@tag", logTag).Caller().Logger()

	deps.logger = &newlog
}

func setupAWS(deps *Dependencies) {
	sublog := deps.logger

	var err error
	deps.awsconfig, err = myaws.AWSConfig("us-east-1")
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to find us-east-1 configuration")
	}

	deps.awssess = myaws.AWSMustConnect("us-east-1", "stockwatch")
	deps.db = myaws.DBMustConnect(deps.awssess, "stockwatch")

	// connect to Dynamo
	deps.ddb, err = myaws.DDBConnect(deps.awssess)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed to connect to DDB")
	}
}

func setupSecrets(deps *Dependencies) {
	sublog := deps.logger
	awssess := deps.awssess

	secrets := make(map[string]string)

	secretValues, err := myaws.AWSGetSecret(awssess, "stockwatch")
	if err != nil {
		sublog.Fatal().Err(err)
	}

	for key := range secretValues {
		value := secretValues[key]
		secrets[key] = value
	}

	deps.secrets = secrets
}

func setupSessionStore(deps *Dependencies) {
	secrets := deps.secrets
	sublog := deps.logger

	var hashKey = []byte(secrets["cookie_auth_key"])
	var blockKey = []byte(secrets["cookie_encryption_key"])
	var secureCookie = securecookie.New(hashKey, blockKey)

	// Initialize session manager and configure the session lifetime -------------
	store, err := dynastore.New(
		dynastore.AWSConfig(deps.awsconfig),
		dynastore.DynamoDB(deps.ddb),
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
	cookieStore := deps.cookieStore
	secrets := deps.secrets

	goth.UseProviders(
		amazon.New(secrets["amazon_api_key"], secrets["amazon_api_secret"], "https://stockwatch.graystorm.com/auth/amazon/callback"),
		facebook.New(secrets["facebook_api_key"], secrets["facebook_api_secret"], "https://stockwatch.graystorm.com/auth/facebook/callback", "email"),
		github.New(secrets["github_api_key"], secrets["github_api_secret"], "https://stockwatch.graystorm.com/auth/github/callback"),
		google.New(secrets["google_oauth_client_id"], secrets["google_oauth_client_secret"], "https://stockwatch.graystorm.com/auth/google/callback", "openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"),
		twitter.New(secrets["twitter_api_key"], secrets["twitter_api_secret"], "https://stockwatch.graystorm.com/auth/twitter/callback"),
	)

	gothic.Store = cookieStore
}

func setupTemplates(deps *Dependencies) {
	sublog := deps.logger

	funcMap := template.FuncMap{
		"Concat":                   Concat,
		"FormatUnixTime":           FormatUnixTime,
		"GradeColor":               GradeColor,
		"SinceColor":               SinceColor,
		"PriceDiffAmt":             PriceDiffAmt,
		"PriceDiffPercAmt":         PriceDiffPercAmt,
		"PriceMoveColorCSS":        PriceMoveColorCSS,
		"PriceBigMoveColorCSS":     PriceBigMoveColorCSS,
		"PriceMoveIndicatorCSS":    PriceMoveIndicatorCSS,
		"PriceBigMoveIndicatorCSS": PriceBigMoveIndicatorCSS,
		"TimeNow":                  TimeNow,
		"ToUpper":                  strings.ToUpper,
		"ToLower":                  strings.ToLower,
	}

	tmpl := template.New("blank").Funcs(funcMap)
	tmpl, err := tmpl.ParseGlob("templates/includes/*.gohtml")
	if err != nil {
		sublog.Fatal().Err(err).Str("template_dir", "templates/includes").Msg("failed to parse include template(s)")
	}
	tmpl, err = tmpl.ParseGlob("templates/modals/*.gohtml")
	if err != nil {
		sublog.Fatal().Err(err).Str("template_dir", "templates/modals").Msg("failed to parse modal template(s)")
	}
	tmpl, err = tmpl.ParseGlob("templates/*.gohtml")
	if err != nil {
		sublog.Fatal().Err(err).Str("template_dir", "templates").Msg("Failed to parse top-level template(s)")
	}

	deps.templates = tmpl
}

func startServer(deps *Dependencies) {
	app := requestHandler{deps: deps}
	sublog := deps.logger

	// starting up web service ---------------------------------------------------
	sublog.Info().Int("port", httpPort).Msg("started serving requests")

	// setup middleware chain
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	router.PathPrefix("/favicon.ico").Handler(http.FileServer(http.Dir("static/images")))

	//router.HandleFunc("/tokensignin", signinHandler()).Methods("POST")
	router.HandleFunc("/auth/{provider}", app.requestHandler(authLoginHandler(deps))).Methods("GET")
	router.HandleFunc("/auth/{provider}/callback", app.requestHandler(authCallbackHandler(deps))).Methods("GET")
	router.HandleFunc("/signout/", app.requestHandler(signoutHandler(deps))).Methods("GET")
	router.HandleFunc("/signout/{provider}", app.requestHandler(signoutHandler(deps))).Methods("GET")
	router.HandleFunc("/logout/", app.requestHandler(signoutHandler(deps))).Methods("GET")
	router.HandleFunc("/logout/{provider}", app.requestHandler(signoutHandler(deps))).Methods("GET")

	router.HandleFunc("/ping", pingHandler()).Methods("GET")
	router.HandleFunc("/internal/cspviolations", app.requestHandler(JSONReportHandler(deps))).Methods("GET")
	router.HandleFunc("/api/v1/{endpoint}", app.requestHandler(apiV1Handler(deps))).Methods("GET")
	router.Handle("/metrics", promhttp.Handler())

	router.HandleFunc("/profile/{status}", app.requestHandler(profileHandler(deps))).Methods("GET")
	router.HandleFunc("/desktop", app.requestHandler(desktopHandler(deps))).Methods("GET")
	router.HandleFunc("/view/{symbol}", app.requestHandler(viewTickerDailyHandler(deps))).Methods("GET")
	router.HandleFunc("/{action:bought|sold}/{symbol}/{acronym}", app.requestHandler(transactionHandler(deps))).Methods("POST")
	router.HandleFunc("/search/{type}", app.requestHandler(searchHandler(deps))).Methods("POST")
	router.HandleFunc("/about", app.requestHandler(homeHandler(deps, "about"))).Methods("GET")
	router.HandleFunc("/terms", app.requestHandler(homeHandler(deps, "terms"))).Methods("GET")
	router.HandleFunc("/privacy", app.requestHandler(homeHandler(deps, "privacy"))).Methods("GET")

	router.HandleFunc("/", app.requestHandler(homeHandler(deps, "home"))).Methods("GET")

	// starup or die
	server := &http.Server{
		Handler:      router,
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
