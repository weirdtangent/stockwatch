package main

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/dgryski/go-skip32"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

func checkAuthState(w http.ResponseWriter, r *http.Request, deps *Dependencies) Watcher {
	webdata := deps.webdata
	sublog := deps.logger
	session := deps.session

	if session.Values["encWatcherId"] != nil {
		encWatcherId := session.Values["encWatcherId"].(string)
		if encWatcherId != "" {
			watcherId := decryptedId(deps, "watcher", encWatcherId)
			watcher, err := getWatcherById(deps, watcherId)
			if err != nil {
				sublog.Error().Err(err).Str("encWatcherId", encWatcherId).Msg("failed to load watcher via encWatcherId {encWatcherId}")
				signoutWatcher(deps)
				return Watcher{}
			}
			if watcher.WatcherStatus != "active" {
				sublog.Error().Err(err).Str("encWatcherId", encWatcherId).Str("status", watcher.WatcherStatus).Msg("watcher is not active: {status}")
				signoutWatcher(deps)
				return Watcher{}
			}

			// setup webdata with watcher-specific values
			webdata["encWatcherId"] = encWatcherId
			webdata["Watcher"] = WebWatcher{watcher.WatcherName, watcher.WatcherStatus, watcher.WatcherLevel, watcher.WatcherTimezone, watcher.WatcherPicURL}

			if watcher.WatcherTimezone != "" {
				_, err = time.LoadLocation(watcher.WatcherTimezone)
				if err == nil {
					webdata["timezone"] = watcher.WatcherTimezone
				}
			}

			if session.Values["provider"] != nil {
				webdata["provider"] = session.Values["provider"].(string)
			}

			return watcher
		}
	}
	sublog.Info().Msg("anonymous visitor")
	webdata["loggedout"] = 1

	return Watcher{}
}

func authLoginHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sublog := deps.logger

		if user, err := gothic.CompleteUserAuth(w, r); err == nil {
			signinUser(deps, w, r, user)
		} else {
			sublog.Error().Err(err).Str("handler", "authLoginHandler").Msg("failed to complete auth")
			gothic.BeginAuthHandler(w, r)
		}
	})
}

func authCallbackHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sublog := deps.logger

		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			sublog.Error().Err(err).Str("handler", "authCallbackHandler").Msg("failed to complete auth")
			deps.messages = append(deps.messages, Message{Text: fmt.Sprintf("Sorry, failed to complete oauth - %s", err), Level: "error"})
			renderTemplate(w, r, deps, "home")
			return
		}
		signinUser(deps, w, r, user)
	})
}

func signinUser(deps *Dependencies, w http.ResponseWriter, r *http.Request, gothUser goth.User) {
	sublog := deps.logger
	session := deps.session

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
		SessionId:       deps.session.ID,
		CreateDatetime:  time.Now(),
		UpdateDatetime:  time.Now(),
	}
	watcher, err := createOrUpdateWatcherFromOAuth(deps, watcher, gothUser.Email)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to get/create watcher from oauth response")
		http.NotFound(w, r)
		return
	}
	if watcher.WatcherId == 0 {
		sublog.Fatal().Msg("watcher should not be undefined here")
	}

	// why does twitter send back a weird gothUser.ExpiresAt?
	if gothUser.ExpiresAt.IsZero() {
		gothUser.ExpiresAt = time.Now().Add(24 * time.Hour)
	}

	oauth := OAuth{
		OAuthId:        0,
		OAuthIssuer:    gothUser.Provider,
		OAuthSub:       gothUser.UserID,
		OAuthIssued:    time.Now(),
		OAuthExpires:   gothUser.ExpiresAt,
		CreateDatetime: time.Now(),
		UpdateDatetime: time.Now(),
	}
	err = oauth.createOrUpdate(deps)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to create/update oauth record")
		http.NotFound(w, r)
		return
	}

	session.Values["encWatcherId"] = encryptId(deps, "watcher", watcher.WatcherId)
	session.Values["provider"] = gothUser.Provider

	// only once do these two dates match - when the watcher is brand new
	if watcher.CreateDatetime == watcher.UpdateDatetime {
		http.Redirect(w, r, "/profile/welcome", http.StatusFound)
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// logout
func signoutHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signoutWatcher(deps)
		gothic.Logout(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
	})
}

func signoutWatcher(deps *Dependencies) {
	session := deps.session
	db := deps.db

	session.Values["encWatcherId"] = ""
	db.Exec("UPDATE watcher SET session_id='' WHERE session_id=?", session.ID)
}

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// random string of bytes, use in nonce values, for example
//   https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
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

func encryptURL(deps *Dependencies, text []byte) ([]byte, error) {
	secrets := deps.secrets

	secret := secrets["next_url_key"]
	key := []byte(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(crand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	cipherstring := ([]byte(base64.URLEncoding.EncodeToString(ciphertext)))
	return cipherstring, nil
}

func decryptURL(deps *Dependencies, cipherstring []byte) ([]byte, error) {
	secrets := deps.secrets

	secret := secrets["next_url_key"]
	key := []byte(secret)
	textstr, err := base64.URLEncoding.DecodeString(string(cipherstring))
	if err != nil {
		return nil, err
	}
	text := ([]byte(textstr))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// split uint64 into high/low uint32s and skip32 them and return as 8 hex chars
func encryptId(deps *Dependencies, objectType string, id uint64) string {
	sublog := deps.logger
	secrets := deps.secrets

	skip64Key := fmt.Sprintf("skip64_%s", objectType)
	key := secrets[skip64Key]
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
	key := secrets[skip64Key]
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
