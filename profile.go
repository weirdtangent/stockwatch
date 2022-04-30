package main

import (
	"database/sql"
	"net/http"
)

type ProfileEmail struct {
	EmailAddress string
	IsPrimary    bool
}

type Profile struct {
	Name           string
	CreateDatetime sql.NullTime
	AvatarURL      string
	Emails         []ProfileEmail
}

func profileHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sublog := deps.logger

		watcher := checkAuthState(w, r, deps)
		if watcher.WatcherId == 0 {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		// messages := deps.messages
		webdata := deps.webdata
		profile, err := getProfile(deps)
		if err != nil {
			sublog.Error().Err(err).Msg("Failed to get profile info")
			// messages = append(messages, Message{fmt.Sprintf("Sorry, error retrieving your profile: %s", err.Error()), "danger"})
		}
		webdata["profile"] = profile

		renderTemplateDefault(w, r, deps, "profile")
	})
}

func getProfile(deps *Dependencies) (*Profile, error) {
	db := deps.db
	sublog := deps.logger
	session := getSession(deps)

	var profile Profile

	watcherId, err := getWatcherIdBySession(deps, session.ID)
	if err != nil {
		sublog.Error().Err(err).Msg("Failed to get profile from session")
		return &profile, err
	}
	watcher, err := getWatcherById(deps, watcherId)
	if err != nil {
		sublog.Error().Err(err).Uint64("watcher_id", watcherId).Msg("Failed to get profile from session")
		return &profile, err
	}

	profile.Name = watcher.WatcherName
	profile.CreateDatetime = watcher.CreateDatetime
	profile.AvatarURL = watcher.WatcherPicURL

	rows, err := db.Queryx("SELECT * FROM watcher_email WHERE watcher_id=? ORDER BY email_is_primary DESC, email_address", watcherId)
	if err != nil {
		sublog.Fatal().Err(err).Str("table_name", "watcher_email").Msg("Failed on SELECT")
	}
	defer rows.Close()

	var watcherEmail WatcherEmail
	emails := make([]ProfileEmail, 0)
	for rows.Next() {
		err = rows.StructScan(&watcherEmail)
		if err != nil {
			sublog.Fatal().Err(err).Str("table_name", "watcher_email").Msg("Error reading result rows")
		}
		emails = append(emails, ProfileEmail{watcherEmail.EmailAddress, watcherEmail.IsPrimary})
	}
	if err := rows.Err(); err != nil {
		sublog.Fatal().Err(err).Str("table_name", "watcher_email").Msg("Error reading result rows")
	}

	profile.Emails = emails
	return &profile, nil
}
