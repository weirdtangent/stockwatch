package main

import (
	"context"
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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/securecookie"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"golang.org/x/oauth2/google"

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"

	"github.com/weirdtangent/myaws"
)

// google one-tap for web
func googleLoginHandler(googleClientId *string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		awssess := ctx.Value("awssess").(*session.Session)
		db := ctx.Value("db").(*sqlx.DB)
		sc := ctx.Value("sc").(*securecookie.SecureCookie)

		// first, make sure csrf token in cookie matches one in body
		csrfToken, err := r.Cookie("g_csrf_token")
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get g_csrf_token")
			http.NotFound(w, r)
			return
		}
		csrfBody := "g_csrf_token=" + r.FormValue("g_csrf_token")
		if len(csrfBody) == 0 || csrfBody != csrfToken.String() {
			logger.Error().Err(err).
				Str("from_cookie", csrfToken.String()).
				Str("from_field", csrfBody).
				Msg("Failed to verify double submit cookie")
			http.NotFound(w, r)
			return
		}

		session := getSession(r)
		id_token := r.FormValue("credential")

		// go get svc account JSON
		google_svc_acct, err := myaws.AWSGetSecretValue(awssess, "stockwatch_google_svc_acct")
		if err != nil {
			logger.Error().Err(err).
				Msg("Failed to retrieve secret")
			http.NotFound(w, r)
			return
		}

		// build our own credentials from that
		credentials, err := google.CredentialsFromJSON(
			context.Background(), []byte(*google_svc_acct),
			"https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile",
		)
		if err != nil {
			logger.Error().Err(err).
				Msg("Failed to build credentials")
			http.NotFound(w, r)
			return
		}

		// create ClientOption with those credentials
		clientOption := option.WithCredentials(credentials)

		// build New Token Validator using that ClientOption
		tokenValidator, err := idtoken.NewValidator(context.Background(), clientOption)
		if err != nil {
			logger.Error().Err(err).
				Msg("Failed to initiate the google idtoken validator")
			http.NotFound(w, r)
			return
		}

		// attempt to validate the idtoken the user presented
		payload, err := tokenValidator.Validate(context.Background(), id_token, *googleClientId)
		if err != nil {
			logger.Error().Err(err).
				Msg("Failed to validate the google idtoken")
			http.NotFound(w, r)
			return
		}

		// get (or create) watcher account based on oauth properties
		var emailAddress = payload.Claims["email"].(string)
		var watcher = &Watcher{0, payload.Claims["name"].(string), emailAddress, "active", "standard", 0, "", ""}
		watcher, err = getOrCreateWatcher(db, watcher)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get/create watcher from one-tap")
			http.NotFound(w, r)
			return
		}

		// get (or write) oauth record tied to watcher until it expires
		var oauth = &OAuth{0, payload.Issuer, payload.IssuedAt, payload.Expires, emailAddress, watcher.WatcherId, payload.Claims["picture"].(string), "active", session.ID, "", "", ""}
		oauth, err = createOrUpdateOAuth(db, oauth)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to create/update oauth record")
			http.NotFound(w, r)
			return
		}

		// now go back and update Watcher record with oauth_id
		watcher.OAuthId = oauth.OAuthId
		watcher.Update(db)

		// set WID session cookie, meaning the user is authenticated and logged-in
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
			logger.Error().Err(err).Msg("Failed to encode cookie")
			return
		}

		// if they came in with `next` param, that says where to go,
		// if the user was on /home, send them to /desktop
		// if they were somewhere else on the site, redirect them back
		// otherwise, they were from off site, send them to /desktop
		nextParam := r.FormValue("next")
		if len(nextParam) > 0 {
			if nextURL, err := decryptURL(awssess, ([]byte(nextParam))); err == nil {
				logger.Info().Str("nextURL", string(nextURL)).Msg("Decoded nextURL found")
				http.Redirect(w, r, string(nextURL), 302)
				return
			} else {
				logger.Error().Str("nextParam", nextParam).Err(err).Msg("Failed to decode next param")
			}
		}
		if ref := r.Referer(); ref == "https://stockwatch.graystorm.com/" {
			http.Redirect(w, r, "/desktop", 302)
			return
		} else if strings.Contains(ref, "https://stockwatch.graystorm.com/") {
			http.Redirect(w, r, ref, 302)
			return
		}
		http.Redirect(w, r, "/desktop", 302)
		return
	})
}

// lougout from google one-tap here
func googleLogoutHandler(googleClientId *string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		sc := ctx.Value("sc").(*securecookie.SecureCookie)
		webdata := ctx.Value("webdata").(map[string]interface{})

		if ok := checkAuthState(w, r); ok {
			var watcher = webdata["watcher"].(*Watcher)
			if encoded, err := sc.Encode("WID", fmt.Sprintf("%d", watcher.WatcherId)); err == nil {
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
				logger.Error().Err(err).Msg("Failed to encode cookie")
			}
		}
		http.Redirect(w, r, "/", 302)
		return
	})
}

// check for WID cookie, set above when authenticated with Google 1-Tap
// plus set some standard webdata keys we'll need for all/most pages
func checkAuthState(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)
	sc := ctx.Value("sc").(*securecookie.SecureCookie)
	webdata := ctx.Value("webdata").(map[string]interface{})
	nonce := ctx.Value("nonce").(string)

	session := getSession(r)
	recents, _ := getRecents(session, r)
	webdata["config"] = ConfigData{}
	webdata["recents"] = Recents{*recents}
	webdata["nonce"] = nonce

	if wid, err := r.Cookie("WID"); err == nil {
		var WIDstr string
		err = sc.Decode("WID", wid.Value, &WIDstr)
		switch err {
		case nil:
			{
				WIDvalue, err := strconv.ParseInt(WIDstr, 10, 64)
				if err != nil {
					logger.Error().Err(err).Str("wid", WIDstr).Msg("Failed to convert cookie value to id")
					break
				}
				watcher, err := getWatcherById(db, WIDvalue)
				if err != nil {
					logger.Error().Err(err).Int64("wid", WIDvalue).Msg("Failed to get watcher via cookie")
					break
				}
				if watcher.WatcherStatus != "active" {
					logger.Error().Err(err).Int64("wid", WIDvalue).Str("watcher_status", watcher.WatcherStatus).Msg("Watcher record not marked 'active'")
					break
				}
				oauth, err := getOAuthByWatcherId(db, WIDvalue)
				if err != nil {
					logger.Error().Err(err).Int64("wid", WIDvalue).Msg("Failed to get oauth record via cookie")
					break
				}
				currentDateTime := time.Now()
				unixTimeNow := currentDateTime.Unix()
				logger.Info().Int64("unix_time", unixTimeNow).Int64("oath_expires", oauth.OAuthExpires).Msg("Checking oauth expiration")
				if unixTimeNow > oauth.OAuthExpires {
					logger.Error().Err(err).Int64("wid", WIDvalue).Msg("OAuth record has expired")
					oauth.Delete(db, WIDvalue)
					if encoded, err := sc.Encode("WID", fmt.Sprintf("%d", watcher.WatcherId)); err == nil {
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
					break
				}
				logger.Info().Msg("Authenticated visitor found")
				webdata["WID"] = wid
				webdata["watcher"] = watcher
				webdata["profilePicURL"] = oauth.PictureURL
				return true
			}
		}
	}
	logger.Info().Msg("Anonymous visitor found")
	webdata["loggedout"] = 1
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

func encryptURL(awssess *session.Session, text []byte) ([]byte, error) {
	secret, err := myaws.AWSGetSecretKV(awssess, "stockwatch_misc", "stockwatch_next_url_key")
	if err != nil {
		return nil, err
	}
	key := []byte(*secret)
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

func decryptURL(awssess *session.Session, cipherstring []byte) ([]byte, error) {
	secret, err := myaws.AWSGetSecretKV(awssess, "stockwatch_misc", "stockwatch_next_url_key")
	if err != nil {
		return nil, err
	}
	key := []byte(*secret)
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
