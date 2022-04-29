package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/dgryski/go-skip32"
	"github.com/gorilla/securecookie"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/rs/zerolog"
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
		ctx := r.Context()
		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to complete auth")
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
		zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to get/create watcher from one-tap")
		http.NotFound(w, r)
		return
	}
	if watcher.WatcherId == 0 {
		zerolog.Ctx(ctx).Fatal().Msg("WatcherId should not be 0 here")
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
		zerolog.Ctx(ctx).Error().Err(err).Msg("failed to create/update oauth record")
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
		zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to encode cookie")
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
		zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to encode cookie (for removal)")
	}
}

// check for WID cookie, set above when authenticated with Google 1-Tap
// plus set some standard webdata keys we'll need for all/most pages
func checkAuthState(w http.ResponseWriter, r *http.Request) (context.Context, Watcher) {
	ctx := r.Context()
	sc := ctx.Value(ContextKey("sc")).(*securecookie.SecureCookie)
	webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})
	nonce := ctx.Value(ContextKey("nonce")).(string)

	session := getSession(r)
	if session.Values["provider"] != nil {
		webdata["provider"] = session.Values["provider"].(string)
	}
	webdata["config"] = ConfigData{}
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
				zerolog.Ctx(ctx).Error().Err(err).Str("wid", WIDstr).Msg("Failed to convert cookie value to id")
				deleteWIDCookie(w, r)
				break
			}
			var watcher Watcher
			err = getWatcherById(ctx, &watcher, WIDvalue)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Uint64("wid", WIDvalue).Msg("Failed to get watcher via cookie")
				deleteWIDCookie(w, r)
				break
			}
			if watcher.WatcherStatus != "active" {
				zerolog.Ctx(ctx).Error().Err(err).Uint64("watcher_id", WIDvalue).Str("watcher_status", watcher.WatcherStatus).Msg("Watcher record not marked 'active'")
				deleteWIDCookie(w, r)
				break
			}
			log := zerolog.Ctx(ctx).With().Str("watcher", encryptId(ctx, "watcher", watcher.WatcherId)).Logger()
			ctx = log.WithContext(ctx)

			zerolog.Ctx(ctx).Info().Str("watcher_status", watcher.WatcherStatus).Msg("authenticated visitor")
			webdata["WID"] = wid
			webdata["watcher"] = watcher

			watcherRecents := getWatcherRecents(ctx, watcher)
			webdata["WatcherRecents"] = watcherRecents

			location, err := time.LoadLocation(watcher.WatcherTimezone)
			if err == nil {
				webdata["tzlocation"] = location
			}

			return ctx, watcher
		}
	}
	// zerolog.Ctx(ctx).Info().Msg("Anonymous visitor found")
	webdata["loggedout"] = 1

	stateStr := session.Values["state"].(string)
	webdata["stateStr"] = stateStr
	webdata["clientId"] = ctx.Value(ContextKey("google_oauth_client_id")).(string)
	webdata["scope"] = "openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile"
	webdata["redirectTo"] = "https://stockwatch.graystorm.com/callback"

	return ctx, Watcher{}
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

// split uint64 into high/low uint32s and skip32 them and return as 8 hex chars
func encryptId(ctx context.Context, objectType string, id uint64) string {
	skip64Key := fmt.Sprintf("skip64_%s", objectType)
	key := ctx.Value(ContextKey(skip64Key))
	if key == nil {
		err := fmt.Errorf("key not found")
		zerolog.Ctx(ctx).Fatal().Str("object", objectType).Err(err).Msg("encryption key not found for {object}")
		return ""
	}
	cipher, err := skip32.New([]byte(key.(string)))
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Int("length", len(key.(string))).Str("object", objectType).Err(err).Msg("encryption failed for {object}")
		return ""
	}

	obfuscated := fmt.Sprintf("%x%x", cipher.Obfus(uint32(id>>32)), cipher.Obfus(uint32(id&0xFFFFFFFF)))
	return obfuscated
}

// break 8 hex chars into high/low uint32s and un-skip32 them and combine to single uint64
func decryptedId(ctx context.Context, objectType string, obfuscated string) uint64 {
	if len(obfuscated) != 16 {
		err := fmt.Errorf("invalid encrypted id")
		zerolog.Ctx(ctx).Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}
	skip64Key := fmt.Sprintf("skip64_%s", objectType)
	key := ctx.Value(ContextKey(skip64Key))
	if key == nil {
		err := fmt.Errorf("key not found")
		zerolog.Ctx(ctx).Fatal().Str("object", objectType).Err(err).Msg("decryption key not found for {object}")
		return 0
	}
	cipher, err := skip32.New([]byte(key.(string)))
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}

	left, err := strconv.ParseUint(obfuscated[:8], 16, 32)
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}
	right, err := strconv.ParseUint(obfuscated[8:16], 16, 32)
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Str("object", objectType).Err(err).Msg("decryption failed for {object}")
		return 0
	}

	id := uint64(cipher.Unobfus(uint32(left)))<<32 | uint64(cipher.Unobfus(uint32(right)))
	return id
}
