package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"

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
func googleLoginHandler(awssess *session.Session, db *sqlx.DB, sc *securecookie.SecureCookie, googleClientId *string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrfToken, err := r.Cookie("g_csrf_token")
		if err != nil {
			log.Error().Err(err).Msg("Failed to get g_csrf_token")
			http.NotFound(w, r)
			return
		}
		csrfBody := "g_csrf_token=" + r.FormValue("g_csrf_token")
		if len(csrfBody) == 0 || csrfBody != csrfToken.String() {
			log.Error().Err(err).
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
			log.Error().Err(err).
				Msg("Failed to retrieve secret")
			http.NotFound(w, r)
			return
		}

		// build our own credentials from that
		credentials, err := google.CredentialsFromJSON(context.Background(), []byte(*google_svc_acct), "https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile")
		if err != nil {
			log.Error().Err(err).
				Msg("Failed to build credentials")
			http.NotFound(w, r)
			return
		}

		// create ClientOption with those credentials
		clientOption := option.WithCredentials(credentials)

		// build New Token Validator using that ClientOption
		tokenValidator, err := idtoken.NewValidator(context.Background(), clientOption)
		if err != nil {
			log.Error().Err(err).
				Msg("Failed to initiate the google idtoken validator")
			http.NotFound(w, r)
			return
		}

		// attempt to validate the idtoken the user presented
		payload, err := tokenValidator.Validate(context.Background(), id_token, *googleClientId)
		if err != nil {
			log.Error().Err(err).
				Msg("Failed to validate the google idtoken")
			http.NotFound(w, r)
			return
		}

		log.Info().Msgf("payload: %#v", payload)

		var profile = GoogleProfileData{
			payload.Claims["name"].(string),
			payload.Claims["given_name"].(string),
			payload.Claims["family_name"].(string),
			payload.Claims["email"].(string),
			payload.Claims["picture"].(string),
			"",
		}
		session.Values["google_profile"] = profile

		// get (or create) watcher account based on oauth properties
		var emailAddress = payload.Claims["email"].(string)
		var watcher = &Watcher{0, payload.Claims["name"].(string), emailAddress, 0, "", ""}
		watcher, err = getOrCreateWatcher(db, watcher)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get/create watcher from one-tap")
			http.NotFound(w, r)
			return
		}

		// get (or write) oauth record tied to watcher until it expires
		var oauth = &OAuth{0, payload.Issuer, payload.IssuedAt, payload.Expires, emailAddress, watcher.WatcherId, payload.Claims["picture"].(string), "active", session.ID, "", "", ""}
		oauth, err = createOrUpdateOAuth(db, oauth)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create/update oauth record")
			http.NotFound(w, r)
			return
		}

		watcher.OAuthId = oauth.OAuthId
		watcher.Update(db)

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
			return
		}

		http.Redirect(w, r, "/desktop", 302)
		return
	})
}

func checkAuthState(r *http.Request, sc *securecookie.SecureCookie, webdata map[string]interface{}) bool {
	session := getSession(r)
	recents, _ := getRecents(session, r)
	webdata["config"] = ConfigData{}
	webdata["recents"] = recents
	webdata["nonce"] = global_nonce

	if wid, err := r.Cookie("WID"); err == nil {
		var value string
		if err = sc.Decode("WID", wid.Value, &value); err == nil {
			webdata["WID"] = value
			return true
		}
	}
	webdata["loggedout"] = 1
	return false
}

//
// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
//
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandStringBytesMask(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; {
		if idx := int(rand.Int63() & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i++
		}
	}
	return string(b)
}
