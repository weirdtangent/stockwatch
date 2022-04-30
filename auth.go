package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/dgryski/go-skip32"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

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
	watcher := Watcher{0, gothUser.UserID, gothUser.Name, "active", "standard", "", gothUser.AvatarURL, session.ID, sql.NullTime{}, sql.NullTime{}}
	sublog.Info().Interface("watcher", watcher).Msg("start off")
	watcher, err := createOrUpdateWatcher(deps, watcher, gothUser.Email)
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
	http.Redirect(w, r, "/desktop", http.StatusFound)
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

// check for WID cookie, set above when authenticated with Google 1-Tap
// plus set some standard webdata keys we'll need for all/most pages
func checkAuthState(w http.ResponseWriter, r *http.Request, deps *Dependencies) Watcher {
	// sc := deps.secureCookie
	webdata := deps.webdata
	nonce := deps.nonce
	session := deps.session
	sublog := deps.logger

	webdata["config"] = ConfigData{}
	webdata["nonce"] = nonce
	webdata["TZLocation"], _ = time.LoadLocation("UTC")

	if session.Values["encWId"] != nil {
		encWId := session.Values["encWId"].(string)
		watcherId := decryptedId(deps, "watcher", encWId)
		watcher, err := getWatcherById(deps, watcherId)
		if err != nil {
			sublog.Error().Err(err).Str("encWId", encWId).Msg("failed to load watcher via encWId {encWId}")
			deleteWIDCookie(w, r, deps)
			return Watcher{}
		}
		if watcher.WatcherStatus != "active" {
			sublog.Error().Err(err).Str("encWId", encWId).Str("status", watcher.WatcherStatus).Msg("watcher is not active: {status}")
			deleteWIDCookie(w, r, deps)
			return Watcher{}
		}

		sublog.Info().Str("encWId", encWId).Msg("authenticated watcher from session")
		webdata["encWId"] = encWId
		webdata["Watcher"] = WebWatcher{watcher.WatcherName, watcher.WatcherStatus, watcher.WatcherLevel, watcher.WatcherTimezone, watcher.WatcherPicURL}

		watcherRecents := getWatcherRecents(deps, watcher)
		webdata["WatcherRecents"] = watcherRecents

		location, err := time.LoadLocation(watcher.WatcherTimezone)
		if err == nil {
			webdata["TZLocation"] = location
		}

		if session.Values["provider"] != nil {
			webdata["provider"] = session.Values["provider"].(string)
		}

		return watcher
	}
	sublog.Info().Msg("anonymous visitor")
	webdata["loggedout"] = 1

	return Watcher{}
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

	obfuscated := fmt.Sprintf("%x%x", cipher.Obfus(uint32(id>>32)), cipher.Obfus(uint32(id&0xFFFFFFFF)))
	return obfuscated
}

// break 8 hex chars into high/low uint32s and un-skip32 them and combine to single uint64
func decryptedId(deps *Dependencies, objectType string, obfuscated string) uint64 {
	sublog := deps.logger
	secrets := deps.secrets

	if len(obfuscated) != 16 {
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

	left, err := strconv.ParseUint(obfuscated[:8], 16, 32)
	if err != nil {
		sublog.Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}
	right, err := strconv.ParseUint(obfuscated[8:16], 16, 32)
	if err != nil {
		sublog.Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}

	id := uint64(cipher.Unobfus(uint32(left)))<<32 | uint64(cipher.Unobfus(uint32(right)))
	return id
}
