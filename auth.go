package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/rs/zerolog/log"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

func authLoginHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, err := gothic.CompleteUserAuth(w, r); err == nil {
			signinUser(w, r, user)
		} else {
			gothic.BeginAuthHandler(w, r)
		}
	})
}

func authCallbackHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			log.Error().Err(err).Msg("Failed to complete auth")
			return
		}
		signinUser(w, r, user)
	})
}

func signinUser(w http.ResponseWriter, r *http.Request, gothUser goth.User) {
	ctx := r.Context()
	session := getSession(r)
	sc := ctx.Value(ContextKey("sc")).(*securecookie.SecureCookie)

	// get (or create) watcher account based on oauth properties
	// specifically, based on the sub value, because email addresses can change
	// and we want a watchers session and "account" to follow them even if they change
	watcher := Watcher{0, gothUser.UserID, gothUser.Name, "active", "standard", "", gothUser.AvatarURL, session.ID, sql.NullTime{}, sql.NullTime{}}
	err := watcher.createOrUpdate(ctx, gothUser.Email)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get/create watcher from one-tap")
		http.NotFound(w, r)
		return
	}
	if watcher.WatcherId == 0 {
		log.Fatal().Msg("WatcherId should not be 0 here")
	}

	// why does twitter send back a weird gothUser.ExpiresAt?
	if gothUser.ExpiresAt.IsZero() {
		gothUser.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	oauth := OAuth{
		0,
		gothUser.Provider,
		gothUser.UserID,
		sql.NullTime{Valid: true, Time: time.Now()},
		sql.NullTime{Valid: true, Time: gothUser.ExpiresAt},
		sql.NullTime{Valid: true, Time: time.Now()},
		sql.NullTime{Valid: true, Time: time.Now()},
	}
	err = oauth.createOrUpdate(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to create/update oauth record")
		http.NotFound(w, r)
		return
	}

	// set WID (WatcherId) session cookie, meaning the user is authenticated and logged-in
	if encoded, err := sc.Encode("WID", fmt.Sprintf("%d", watcher.WatcherId)); err == nil {
		cookie := &http.Cookie{
			Name:     "WID",
			Value:    encoded,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
		}
		http.SetCookie(w, cookie)
	} else {
		log.Error().Err(err).Msg("Failed to encode cookie")
	}

	session.Values["provider"] = gothUser.Provider
	http.Redirect(w, r, "/desktop", http.StatusFound)
}

// logout from google one-tap here
func signoutHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deleteWIDCookie(w, r)
		gothic.Logout(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
	})
}

func deleteWIDCookie(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sc := ctx.Value(ContextKey("sc")).(*securecookie.SecureCookie)

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
	} else {
		log.Error().Err(err).Msg("Failed to encode cookie (for removal)")
	}
}

// check for WID cookie, set above when authenticated with Google 1-Tap
// plus set some standard webdata keys we'll need for all/most pages
func checkAuthState(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	sc := ctx.Value(ContextKey("sc")).(*securecookie.SecureCookie)
	webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})
	nonce := ctx.Value(ContextKey("nonce")).(string)

	session := getSession(r)
	recents, _ := getRecents(session, r)
	if session.Values["provider"] != nil {
		webdata["provider"] = session.Values["provider"].(string)
	}
	webdata["config"] = ConfigData{}
	webdata["recents"] = *recents
	webdata["nonce"] = nonce
	location, _ := time.LoadLocation("UTC")
	webdata["tzlocation"] = location

	if wid, err := r.Cookie("WID"); err == nil {
		var WIDstr string
		err = sc.Decode("WID", wid.Value, &WIDstr)
		switch err {
		case nil:
			WIDvalue, err := strconv.ParseUint(WIDstr, 10, 64)
			if err != nil {
				log.Error().Err(err).Str("wid", WIDstr).Msg("Failed to convert cookie value to id")
				deleteWIDCookie(w, r)
				break
			}
			var watcher Watcher
			err = getWatcherById(ctx, &watcher, WIDvalue)
			if err != nil {
				log.Error().Err(err).Uint64("wid", WIDvalue).Msg("Failed to get watcher via cookie")
				deleteWIDCookie(w, r)
				break
			}
			if watcher.WatcherStatus != "active" {
				log.Error().Err(err).Uint64("watcher_id", WIDvalue).Str("watcher_status", watcher.WatcherStatus).Msg("Watcher record not marked 'active'")
				deleteWIDCookie(w, r)
				break
			}
			//oauth, err := getOAuthBySub(ctx, watcher.WatcherSub)
			//if err != nil {
			//	log.Error().Err(err).Int64("watcher_id", WIDvalue).Msg("Failed to get oauth record by sub")
			//	break
			//}
			//currentDateTime := time.Now()
			//unixTimeNow := currentDateTime.Unix()
			//log.Info().Int64("unix_time", unixTimeNow).Int64("oath_expires", oauth.OAuthExpires).Msg("Checking oauth expiration")
			//if unixTimeNow > oauth.OAuthExpires {
			//	log.Warn().Int64("watcher_id", WIDvalue).Msg("OAuth record has expired")
			//}
			log.Info().Uint64("watcher_id", watcher.WatcherId).Str("watcher_status", watcher.WatcherStatus).Msg("authenticated visitor")
			webdata["WID"] = wid
			webdata["watcher"] = watcher

			location, err := time.LoadLocation(watcher.WatcherTimezone)
			if err == nil {
				webdata["tzlocation"] = location
			}

			return true
		}
	}
	log.Info().Msg("Anonymous visitor found")
	webdata["loggedout"] = 1

	stateStr := session.Values["state"].(string)
	webdata["stateStr"] = stateStr
	webdata["clientId"] = ctx.Value(ContextKey("google_oauth_client_id")).(string)
	webdata["scope"] = "openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"
	webdata["redirectTo"] = "https://stockwatch.graystorm.com/callback"

	return false
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

// func encryptURL(ctx context.Context, text []byte) ([]byte, error) {
// 	secret := ctx.Value(ContextKey("next_url_key")).(string)
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

// func decryptURL(ctx context.Context, cipherstring []byte) ([]byte, error) {
// 	secret := ctx.Value(ContextKey("next_url_key")).(string)
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
