package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/sessions"
)

var (
	forwardedRE      = regexp.MustCompile(`for=(.*)`)
	skipLoggingPaths = regexp.MustCompile(`^/(ping|metrics|static|favicon.ico)`)
	obfuscateParams  = regexp.MustCompile(`(token|verifier|pwd|password|code|state)=([^\&]+)`)
)

// withContext middleware -----------------------------------------------------
type AddContext struct {
	handler http.Handler
	deps    *Dependencies
}

type ContextKey string

func (ac *AddContext) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// lets add request_id to this context and as a response header
	// but also as a cookie with a short expiration so we can catch
	// additional immediate requests with the same id
	reqHeader := r.Header
	resHeader := w.Header()

	var requestId string
	ridCookie, err := r.Cookie("RID")
	if err == nil {
		requestId = ridCookie.Value
	}
	if len(requestId) == 0 {
		requestId = reqHeader.Get("X-Request-ID")
	}
	resHeader.Set("X-Request-ID", requestId)
	ac.deps.request_id = requestId

	// write/update cookie with RID
	ridCookie = &http.Cookie{
		Name:     "RID",
		Value:    requestId,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(5 * time.Second), // so, requests within 5 seconds will have the same request_id
	}
	http.SetCookie(w, ridCookie)

	// newlog := ac.deps.logger.With().Str("request_id", requestId).Logger()
	// ac.deps.logger = &newlog

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

	ac.deps.redisPool = redisPool
	ac.deps.config = defaultConfig
	ac.deps.webdata = make(map[string]interface{})

	ac.deps.nonce = RandStringMask(32)

	ac.handler.ServeHTTP(w, r)
}
func withContext(h http.Handler, deps *Dependencies) *AddContext {
	return &AddContext{h, deps}
}

// withLogging middleware -----------------------------------------------------
type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

type Logger struct {
	handler http.Handler
	deps    *Dependencies
}

func (l *Logger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	sublog := l.deps.logger

	recorder := &StatusRecorder{ResponseWriter: w, Status: 200}

	// handle the HTTP request
	l.handler.ServeHTTP(recorder, r)

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

		sublog.Info().
			Str("method", r.Method).
			Str("url", cleanURL).
			Int("status_code", recorder.Status).
			Str("remote_ip_addr", remote_ip_addr).
			Int64("response_time", time.Since(t).Nanoseconds()).
			Msg("request")
	}
}
func withLogging(h http.Handler, deps *Dependencies) *Logger {
	return &Logger{h, deps}
}

// withExtraHeader middleware -------------------------------------------------

type ExtraHeader struct {
	handler http.Handler
	deps    *Dependencies
}

func (ah *ExtraHeader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nonce := ah.deps.nonce

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
func withExtraHeader(h http.Handler, deps *Dependencies) *ExtraHeader {
	return &ExtraHeader{h, deps}
}

// withSession management middleware ------------------------------------------

type Session struct {
	handler http.Handler
	deps    *Dependencies
}

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sublog := s.deps.logger

	session, err := s.deps.cookieStore.Get(r, "SID")
	if err != nil {
		sublog.Fatal().Err(err).Msg("Failed to get/create session")
	}
	if session.IsNew {
		state := RandStringMask(32)
		session.Values["state"] = state
		session.Values["theme"] = "dark"
		err := session.Save(r, w)
		if err != nil {
			sublog.Fatal().Err(err).Msg("Failed to save session")
		}
	}
	s.deps.session = session

	defer session.Save(r, w)

	s.handler.ServeHTTP(w, r)
}

func withSession(h http.Handler, deps *Dependencies) *Session {
	return &Session{h, deps}
}

func getSession(deps *Dependencies) *sessions.Session {
	sublog := deps.logger

	session := deps.session
	if session == nil {
		sublog.Fatal().Err(fmt.Errorf("failed to get session from context")).Msg("")
	}
	return session
}
