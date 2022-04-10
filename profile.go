package main

import (
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type ProfileEmail struct {
	EmailAddress string
	IsPrimary    bool
}

type Profile struct {
	Name           string
	CreateDatetime string
	AvatarURL      string
	Emails         []ProfileEmail
}

func profileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		messages := ctx.Value(ContextKey("messages")).(*[]Message)
		webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})
		logger := log.Ctx(ctx)

		if ok := checkAuthState(w, r); !ok {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		profile, err := getProfile(r)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get profile info")
			*messages = append(*messages, Message{fmt.Sprintf("Sorry, error retrieving your profile: %s", err.Error()), "danger"})
		}
		webdata["profile"] = profile
		renderTemplateDefault(w, r, "profile")
	})
}

func getProfile(r *http.Request) (*Profile, error) {
	ctx := r.Context()
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)
	session := getSession(r)

	var profile Profile

	watcherId, err := getWatcherIdBySession(ctx, session.ID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get profile from session")
		return &profile, err
	}
	var watcher Watcher
	err = getWatcherById(ctx, &watcher, watcherId)
	if err != nil {
		logger.Error().Err(err).Int64("watcher_id", watcherId).Msg("Failed to get profile from session")
		return &profile, err
	}

	profile.Name = watcher.WatcherName
	profile.CreateDatetime = watcher.CreateDatetime
	profile.AvatarURL = watcher.WatcherPicURL

	rows, err := db.Queryx("SELECT * FROM watcher_email WHERE watcher_id=? ORDER BY email_is_primary DESC, email_address", watcherId)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "watcher_email").
			Msg("Failed on SELECT")
	}
	defer rows.Close()

	var watcherEmail WatcherEmail
	emails := make([]ProfileEmail, 0)
	for rows.Next() {
		err = rows.StructScan(&watcherEmail)
		if err != nil {
			logger.Fatal().Err(err).
				Str("table_name", "watcher_email").
				Msg("Error reading result rows")
		}
		emails = append(emails, ProfileEmail{watcherEmail.EmailAddress, watcherEmail.IsPrimary})
	}
	if err := rows.Err(); err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "watcher_email").
			Msg("Error reading result rows")
	}

	profile.Emails = emails
	return &profile, nil
}
