package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dgryski/go-skip32"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/rs/zerolog"
)

// Sets up for a web request - anything but an internal handler will HAVE to call this first and take the new "deps"
//   also, check for WID cookie, set above when authenticated with Google 1-Tap
//   plus set some standard webdata keys we'll need for all/most pages
func checkAuthState(w http.ResponseWriter, r *http.Request, deps *Dependencies) (Watcher, *Dependencies) {
	// here we make a copy of the permenant "deps" but with new versions of what might change during THIS request:
	// new request_id, new nonce, new logger, new config, new webdata
	// so we are not overwriting the same deps addresses that other web requests are also updating
	resHeader := w.Header()
	newnonce := resHeader.Get("X-Nonce")
	newrequestid := resHeader.Get("X-Request-ID")
	newlog := zerolog.New(os.Stdout).With().Str("request-id", newrequestid).Logger()
	newconfig := make(map[string]interface{})
	newwebdata := make(map[string]interface{})
	newdeps := Dependencies{
		awssess:      deps.awssess,
		db:           deps.db,
		secureCookie: deps.secureCookie,
		cookieStore:  deps.cookieStore,
		redisPool:    deps.redisPool,
		secrets:      deps.secrets,
		session:      deps.session,
		request_id:   newrequestid,
		nonce:        newnonce,
		logger:       &newlog,
		config:       newconfig,
		webdata:      newwebdata,
	}

	config := newdeps.config
	config["is_market_open"] = isMarketOpen()
	config["quote_refresh"] = 20
	newdeps.config = config

	webdata := newdeps.webdata
	webdata["nonce"] = newnonce
	webdata["user-timezone"] = "UTC"
	webdata["request-id"] = newdeps.request_id

	sublog := newdeps.logger
	session := newdeps.session

	if session.Values["encWId"] != nil {
		encWId := session.Values["encWId"].(string)
		watcherId := decryptedId(deps, "watcher", encWId)
		watcher, err := getWatcherById(deps, watcherId)
		if err != nil {
			sublog.Error().Err(err).Str("encWId", encWId).Msg("failed to load watcher via encWId {encWId}")
			deleteWIDCookie(w, r, deps)
			return Watcher{}, &newdeps
		}
		if watcher.WatcherStatus != "active" {
			sublog.Error().Err(err).Str("encWId", encWId).Str("status", watcher.WatcherStatus).Msg("watcher is not active: {status}")
			deleteWIDCookie(w, r, deps)
			return Watcher{}, &newdeps
		}

		sublog.Info().Str("encWId", encWId).Msg("authenticated watcher from session")
		webdata["encWId"] = encWId
		webdata["Watcher"] = WebWatcher{watcher.WatcherName, watcher.WatcherStatus, watcher.WatcherLevel, watcher.WatcherTimezone, watcher.WatcherPicURL}

		watcherRecents := getWatcherRecents(deps, watcher)
		webdata["WatcherRecents"] = watcherRecents

		if watcher.WatcherTimezone != "" {
			_, err = time.LoadLocation(watcher.WatcherTimezone)
			if err == nil {
				webdata["timezone"] = watcher.WatcherTimezone
			}
		}

		if session.Values["provider"] != nil {
			webdata["provider"] = session.Values["provider"].(string)
		}

		return watcher, &newdeps
	}
	sublog.Info().Msg("anonymous visitor")
	webdata["loggedout"] = 1

	return Watcher{}, &newdeps
}

func authLoginHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, err := gothic.CompleteUserAuth(w, r); err == nil {
			signinUser(deps, w, r, user)
		} else {
			gothic.BeginAuthHandler(w, r)
		}
	})
}

func authCallbackHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sublog := deps.logger

		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			sublog.Error().Err(err).Msg("Failed to complete auth")
			return
		}
		signinUser(deps, w, r, user)
	})
}

func signinUser(deps *Dependencies, w http.ResponseWriter, r *http.Request, gothUser goth.User) {
	session := deps.session
	// sc := deps.secureCookie
	sublog := deps.logger

	// get (or create) watcher account based on oauth properties
	// specifically, based on the oauth_sub value, because email addresses can change
	// and we want a watchers session and "account" to follow them even if they change
	watcher := Watcher{
		WatcherId:       0,
		WatcherSub:      gothUser.UserID,
		WatcherName:     gothUser.Name,
		WatcherNickname: gothUser.Name,
		WatcherStatus:   "active",
		WatcherLevel:    "standard",
		WatcherTimezone: "",
		WatcherPicURL:   gothUser.AvatarURL,
		SessionId:       session.ID,
		CreateDatetime:  sql.NullTime{},
		UpdateDatetime:  sql.NullTime{},
	}
	watcher, err := createOrUpdateWatcherFromOAuth(deps, watcher, gothUser.Email)
	if err != nil {
		sublog.Error().Err(err).Msg("Failed to get/create watcher from one-tap")
		http.NotFound(w, r)
		return
	}
	if watcher.WatcherId == 0 {
		sublog.Fatal().Msg("WatcherId should not be 0 here")
	}

	// why does twitter send back a weird gothUser.ExpiresAt?
	if gothUser.ExpiresAt.IsZero() {
		gothUser.ExpiresAt = time.Now().Add(24 * time.Hour)
	}

	oauth := OAuth{
		OAuthId:        0,
		OAuthIssuer:    gothUser.Provider,
		OAuthSub:       gothUser.UserID,
		OAuthIssued:    sql.NullTime{Valid: true, Time: time.Now()},
		OAuthExpires:   sql.NullTime{Valid: true, Time: gothUser.ExpiresAt},
		CreateDatetime: sql.NullTime{Valid: true, Time: time.Now()},
		UpdateDatetime: sql.NullTime{Valid: true, Time: time.Now()},
	}
	err = oauth.createOrUpdate(deps)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to create/update oauth record")
		http.NotFound(w, r)
		return
	}

	// set WID (WatcherId) session cookie, meaning the user is authenticated and logged-in
	// if encoded, err := sc.Encode("WID", fmt.Sprintf("%d", watcher.WatcherId)); err == nil {
	// 	widCookie := &http.Cookie{
	// 		Name:     "WID",
	// 		Value:    encoded,
	// 		Path:     "/",
	// 		Secure:   true,
	// 		HttpOnly: true,
	// 		SameSite: http.SameSiteStrictMode,
	// 	}
	// 	http.SetCookie(w, widCookie)
	// } else {
	// 	sublog.Error().Err(err).Msg("Failed to encode cookie")
	// }

	session.Values["encWId"] = encryptId(deps, "watcher", watcher.WatcherId)
	session.Values["provider"] = gothUser.Provider
	// only once do these two dates match - when the watcher is brand new
	if watcher.CreateDatetime == watcher.UpdateDatetime {
		http.Redirect(w, r, "/profile/welcome", http.StatusFound)
	} else {
		http.Redirect(w, r, "/desktop", http.StatusFound)
	}
}

// logout from google one-tap here
func signoutHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deleteWIDCookie(w, r, deps)
		gothic.Logout(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
	})
}

func deleteWIDCookie(w http.ResponseWriter, r *http.Request, deps *Dependencies) {
	sc := deps.secureCookie

	if encoded, err := sc.Encode("WID", "invalid"); err == nil {
		cookie := &http.Cookie{
			Name:     "WID",
			Value:    encoded,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			MaxAge:   -1,
		}
		http.SetCookie(w, cookie)
	}
}

// random string of bytes, use in nonce values, for example
//   https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandStringMask(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; {
		if idx := int(rand.Int63() & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i++
		}
	}
	return string(b)
}

// func encryptURL(deps *Dependencies, text []byte) ([]byte, error) {
// 	secret := secrets["next_url_key"]
// 	key := []byte(secret)
// 	block, err := aes.NewCipher(key)
// 	if err != nil {
// 		return nil, err
// 	}
// 	b := base64.StdEncoding.EncodeToString(text)
// 	ciphertext := make([]byte, aes.BlockSize+len(b))
// 	iv := ciphertext[:aes.BlockSize]
// 	if _, err := io.ReadFull(crand.Reader, iv); err != nil {
// 		return nil, err
// 	}
// 	cfb := cipher.NewCFBEncrypter(block, iv)
// 	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
// 	cipherstring := ([]byte(base64.URLEncoding.EncodeToString(ciphertext)))
// 	return cipherstring, nil
// }

// func decryptURL(deps *Dependencies, cipherstring []byte) ([]byte, error) {
// 	secret := secrets["next_url_key"]
// 	key := []byte(secret)
// 	textstr, err := base64.URLEncoding.DecodeString(string(cipherstring))
// 	if err != nil {
// 		return nil, err
// 	}
// 	text := ([]byte(textstr))
// 	block, err := aes.NewCipher(key)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(text) < aes.BlockSize {
// 		return nil, errors.New("ciphertext too short")
// 	}
// 	iv := text[:aes.BlockSize]
// 	text = text[aes.BlockSize:]
// 	cfb := cipher.NewCFBDecrypter(block, iv)
// 	cfb.XORKeyStream(text, text)
// 	data, err := base64.StdEncoding.DecodeString(string(text))
// 	if err != nil {
// 		return nil, err
// 	}
// 	return data, nil
// }

// split uint64 into high/low uint32s and skip32 them and return as 8 hex chars
func encryptId(deps *Dependencies, objectType string, id uint64) string {
	sublog := deps.logger
	secrets := deps.secrets

	skip64Key := fmt.Sprintf("skip64_%s", objectType)
	key := *secrets[skip64Key]
	if key == "" {
		err := fmt.Errorf("key not found")
		sublog.Fatal().Str("object", objectType).Err(err).Msg("encryption key not found for {object}")
		return ""
	}
	cipher, err := skip32.New([]byte(key))
	if err != nil {
		sublog.Fatal().Int("length", len(key)).Str("object", objectType).Err(err).Msg("encryption failed for {object}")
		return ""
	}

	obfuscated := ""
	if (id >> 32) != 0 {
		obfuscated = fmt.Sprintf("%x%x", cipher.Obfus(uint32(id>>32)), cipher.Obfus(uint32(id&0xFFFFFFFF)))
	} else {
		obfuscated = fmt.Sprintf("%x", cipher.Obfus(uint32(id&0xFFFFFFFF)))
	}

	return obfuscated
}

// break 8 hex chars into high/low uint32s and un-skip32 them and combine to single uint64
func decryptedId(deps *Dependencies, objectType string, obfuscated string) uint64 {
	sublog := deps.logger
	secrets := deps.secrets

	if len(obfuscated) != 8 && len(obfuscated) != 16 {
		err := fmt.Errorf("invalid encrypted id")
		sublog.Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}
	skip64Key := fmt.Sprintf("skip64_%s", objectType)
	key := *secrets[skip64Key]
	if key == "" {
		err := fmt.Errorf("key not found")
		sublog.Fatal().Str("object", objectType).Err(err).Msg("decryption key not found for {object}")
		return 0
	}
	cipher, err := skip32.New([]byte(key))
	if err != nil {
		sublog.Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}

	var left, right, id uint64
	left, err = strconv.ParseUint(obfuscated[:8], 16, 32)
	if err != nil {
		sublog.Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}
	if len(obfuscated) == 16 {
		right, err = strconv.ParseUint(obfuscated[8:16], 16, 32)
		if err != nil {
			sublog.Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
			return 0
		}
		id = uint64(cipher.Unobfus(uint32(left)))<<32 | uint64(cipher.Unobfus(uint32(right)))
	} else {
		id = uint64(cipher.Unobfus(uint32(left)))
	}

	return id
}
