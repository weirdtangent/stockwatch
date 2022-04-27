package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/savaki/dynastore"
)

var (
	forwardedRE      = regexp.MustCompile(`for=(.*)`)
	skipLoggingPaths = regexp.MustCompile(`^/(ping|metrics|static|favicon.ico)`)
	obfuscateParams  = regexp.MustCompile(`(token|verifier|pwd|password)=([^\&]+)`)
)

// AddContext middleware ------------------------------------------------------
type AddContext struct {
	handler http.Handler
	awssess *session.Session
	db      *sqlx.DB
	sc      *securecookie.SecureCookie
	secrets map[string]string
}

type ContextKey string

func (ac *AddContext) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	reqHeader := r.Header
	resHeader := w.Header()

	// lets add request_id to this context and as a response header
	// but also as a cookie with a short expiration so we can catch
	// additional immediate requests with the same id
	var rid string
	ridCookie, err := r.Cookie("RID")
	if err == nil {
		rid = ridCookie.Value
	}
	if len(rid) == 0 {
		rid = reqHeader.Get("X-Request-ID")
	}
	ctx = context.WithValue(ctx, ContextKey("request_id"), rid)
	resHeader.Set("X-Request-ID", rid)

	ridCookie = &http.Cookie{
		Name:     "RID",
		Value:    rid,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		Expires:  time.Now().Add(3 * time.Second),
	}
	http.SetCookie(w, ridCookie)

	logger := zerolog.Ctx(ctx)
	logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Str("request_id", rid)
	})

	messages := make([]Message, 0)

	// redis connection
	redisPool := &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", "localhost:6379")
		},
	}

	defaultConfig := make(map[string]interface{})
	defaultConfig["is_market_open"] = isMarketOpen()
	defaultConfig["quote_refresh"] = 20

	r = r.Clone(context.WithValue(r.Context(), ContextKey("awssess"), ac.awssess))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("db"), ac.db))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("sc"), ac.sc))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("redisPool"), redisPool))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("google_oauth_client_id"), ac.secrets["google_oauth_client_id"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("google_oauth_client_secret"), ac.secrets["google_oauth_secret"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("github_oauth_key"), ac.secrets["github_oauth_key"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("google_svc_acct"), ac.secrets["stockwatch_google_svc_acct"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("yhfinance_apikey"), ac.secrets["yhfinance_rapidapi_key"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("yhfinance_apihost"), ac.secrets["yhfinance_rapidapi_host"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("msfinance_apikey"), ac.secrets["msfinance_rapidapi_key"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("msfinance_apihost"), ac.secrets["msfinance_rapidapi_host"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("bbfinance_apikey"), ac.secrets["bbfinance_rapidapi_key"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("bbfinance_apihost"), ac.secrets["bbfinance_rapidapi_host"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("skip64_watcher"), ac.secrets["skip64_watcher"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("next_url_key"), ac.secrets["next_url_key"]))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("config"), defaultConfig))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("webdata"), make(map[string]interface{})))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("messages"), &messages))
	r = r.Clone(context.WithValue(r.Context(), ContextKey("nonce"), RandStringMask(32)))

	ac.handler.ServeHTTP(w, r)
}

func withAddContext(h http.Handler, awssess *session.Session, db *sqlx.DB, sc *securecookie.SecureCookie, secrets map[string]string) *AddContext {
	return &AddContext{h, awssess, db, sc, secrets}
}

// AddHeaders middleware ------------------------------------------------------

type AddHeader struct {
	handler http.Handler
}

func (ah *AddHeader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nonce := ctx.Value(ContextKey("nonce")).(string)

	resHeader := w.Header()
	csp := map[string][]string{
		"base-uri":    {"'self'"},
		"default-src": {"'self'"},
		"connect-src": {"'self'", "accounts.google.com", "www.google-analytics.com", "*.fontawesome.com", "api.amazon.com", "*.facebook.com"},
		"style-src":   {"'self'", "fonts.googleapis.com", "accounts.google.com", "'unsafe-inline'"},
		"script-src":  {"'self'", "apis.google.com", "www.googletagmanager.com", "accounts.google.com", "kit.fontawesome.com", "assets.loginwithamazon.com", "*.facebook.net", "'nonce-" + nonce + "'"},
		"img-src":     {"* data:"}, // 'self' data: *.googleusercontent.com *.twimg.com avatars.githubusercontent.com assets.bwbx.io im.mstar.com im.morningstar.com mma.prnewswire.com",
		"font-src":    {"'self'", "fonts.gstatic.com", "*.fontawesome.com"},
		"frame-src":   {"'self'", "accounts.google.com", "*.amazon.com", "*.facebook.com"},
		"object-src":  {"'none'"},
		"report-uri":  {"/internal/cspviolations"},
		"report-to":   {"default"},
	}
	cspString := ""
	for category := range csp {
		cspString += fmt.Sprintf("%s %s;\n", category, strings.Join(csp[category], " "))
	}
	resHeader.Set("Content-Security-Policy", cspString)

	reportTo := `{"group":"default","max-age":1800,"endpoints":[{"url":"https://stockwatch.graystorm.com/internal/cspviolations"}],"include_subdomains":true}`
	resHeader.Set("Report-To", reportTo)

	ah.handler.ServeHTTP(w, r)
}
func withAddHeader(h http.Handler) *AddHeader {
	return &AddHeader{h}
}

// Logging middleware ---------------------------------------------------------

type Logger struct {
	handler http.Handler
}

func (l *Logger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()

	// attach zerolog to request context
	logTag := "stockwatch"
	log := log.With().Str("@tag", logTag).Caller().Logger()
	r = r.WithContext(log.WithContext(r.Context()))
	ctx := r.Context()

	// handle the HTTP request
	l.handler.ServeHTTP(w, r)

	// don't logs these, no reason to
	if !skipLoggingPaths.MatchString(r.URL.String()) {
		ForwardedHdrs := r.Header["Forwarded"]
		remote_ip_addr := ""
		if len(ForwardedHdrs) > 0 {
			submatches := forwardedRE.FindStringSubmatch(ForwardedHdrs[0])
			if len(submatches) >= 1 {
				remote_ip_addr = submatches[1]
			}
		}

		cleanURL := r.URL.String()
		cleanURL = obfuscateParams.ReplaceAllString(cleanURL, "$1=xxxxxx")

		zerolog.Ctx(ctx).Info().Str("url", cleanURL).Int("status_code", 200).Str("method", r.Method).Str("remote_ip_addr", remote_ip_addr).Int64("response_time", time.Since(t).Nanoseconds()).Msg("request")
	}
}
func withLogging(h http.Handler) *Logger {
	return &Logger{handler: h}
}

// Session management middleware ----------------------------------------------

type Session struct {
	store   *dynastore.Store
	handler http.Handler
}

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, err := s.store.Get(r, "SID")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("Failed to get/create session")
	}
	if session.IsNew {
		state := RandStringMask(32)
		session.Values["state"] = state
		session.Values["recents"] = []string{}
		session.Values["theme"] = "dark"
		err := session.Save(r, w)
		if err != nil {
			zerolog.Ctx(ctx).Fatal().Err(err).Msg("Failed to save session")
		}
	}
	r = r.Clone(context.WithValue(r.Context(), ContextKey("ddbs"), session))

	defer session.Save(r, w)

	s.handler.ServeHTTP(w, r)
}
func withSession(store *dynastore.Store, h http.Handler) *Session {
	return &Session{store, h}
}

func getSession(r *http.Request) *sessions.Session {
	ctx := r.Context()
	session := ctx.Value(ContextKey("ddbs")).(*sessions.Session)
	if session == nil {
		zerolog.Ctx(ctx).Fatal().Err(fmt.Errorf("failed to get session from context")).Msg("")
	}
	return session
}
