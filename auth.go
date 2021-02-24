package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/rs/zerolog/log"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"

	"github.com/weirdtangent/myaws"
)

func googleLoginHandler(oauthConfig *oauth2.Config, oauthStateString string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := oauthConfig.AuthCodeURL(oauthStateString)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})
}

// google authorized redirect URL is /callback and lands here
func googleCallbackHandler(oauthConfig *oauth2.Config, oauthStateString string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := getUserInfo(oauthConfig, oauthStateString, r.FormValue("state"), r.FormValue("code"))
		if err != nil {
			log.Error().Err(err).
				Msg("Failed to get google user info")
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		}
		return
	})
}

func getUserInfo(oauthConfig *oauth2.Config, oauthStateString string, state string, code string) ([]byte, error) {
	if state != oauthStateString {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := oauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	return contents, nil
}

// validate idtoken the user has
func googleTokenSigninHandler(aws *session.Session, googleClientId *string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)
		id_token := r.FormValue("idtoken")

		// go get svc account JSON
		google_svc_acct, err := myaws.AWSGetSecretValue(aws, "stockwatch_google_svc_acct")
		if err != nil {
			log.Fatal().Err(err).
				Msg("Failed to retrieve secret")
		}

		// build our own credentials from that
		credentials, err := google.CredentialsFromJSON(context.Background(), []byte(*google_svc_acct), "https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile")
		if err != nil {
			log.Fatal().Err(err).
				Msg("Failed to build credentials")
		}

		// create ClientOption with those credentials
		clientOption := option.WithCredentials(credentials)

		// build New Token Validator using that ClientOption
		tokenValidator, err := idtoken.NewValidator(context.Background(), clientOption)
		if err != nil {
			log.Error().Err(err).
				Msg("Failed to initiate the google idtoken validator")
		}

		// attempt to validate the idtoken the user presented
		payload, err := tokenValidator.Validate(context.Background(), id_token, *googleClientId)
		if err != nil {
			log.Error().Err(err).
				Msg("Failed to validate the google idtoken")
		}

		var profile = GoogleProfileData{
			payload.Claims["name"].(string),
			payload.Claims["given_name"].(string),
			payload.Claims["family_name"].(string),
			payload.Claims["email"].(string),
			payload.Claims["picture"].(string),
			payload.Claims["locale"].(string),
		}
		session.Values["google_profile"] = profile

		return
	})
}

//"&{accounts.google.com 602086455575-vj2spkou0ucsntgol9srfkf8mka3o5i2.apps.googleusercontent.com 1614054670 1614051070 101798707958613429940
//map[
//  at_hash:uF3FC0IOxik0KUo_oKE1CQ
//  aud:602086455575-vj2spkou0ucsntgol9srfkf8mka3o5i2.apps.googleusercontent.com
//  azp:602086455575-vj2spkou0ucsntgol9srfkf8mka3o5i2.apps.googleusercontent.com
//  email:jeff.culverhouse@gmail.com
//  email_verified:true
//  exp:1.61405467e+09
//  family_name:Culverhouse
//  given_name:Jeff
//  iat:1.61405107e+09
//  iss:accounts.google.com
//  jti:0b13b7193521d543d53d08b6f4a26733206cfd9b
//  locale:en
//  name:Jeff Culverhouse
//  picture:https://lh3.googleusercontent.com/a-/AOh14Gg9YEhlPdLTn8fy0LD4vjISBUy-pZcI6heki1aEeg=s96-c
//  sub:101798707958613429940
//]}
