package main

import (
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
)

type ProfileEmail struct {
	EmailAddress string
	IsPrimary    bool
}

type Profile struct {
	Name           string
	Nickname       string
	Timezone       string
	AvatarURL      string
	CreateDatetime time.Time
	Emails         []ProfileEmail
}

func profileHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata
		sublog := deps.logger

		watcher := checkAuthState(w, r, deps)
		if watcher.WatcherId == 0 {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		params := mux.Vars(r)
		status := params["status"]

		if status == "welcome" {
			webdata["announcement"] = []string{`
			Welcome to Stockwatch! This is really just a hobby site for me to learn Go programming, but I
			encourage feedback to let me know cool stuff I should try! I need to work on a feedback form,
			but meanwhile you can just email request@graystorm.com. I suspect, though, I will most often be
			thinking, "yeah, I dunno how to do that" ;) Also, these profile settings below don't quite
			work yet, but I'm working on it!`}
			updateWatcher(deps, watcher)
		}

		profile, err := getProfile(deps, watcher)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to get profile info")
		}
		webdata["profile"] = profile

		timezones := getTimezones(deps)

		sort.Slice(timezones, func(i, j int) bool {
			return timezones[i].Location < timezones[j].Location
		})

		webdata["timezones"] = timezones

		renderTemplate(w, r, deps, "profile")
	})
}

func getProfile(deps *Dependencies, watcher Watcher) (*Profile, error) {
	db := deps.db
	sublog := deps.logger

	var profile Profile

	profile.Name = watcher.WatcherName
	profile.Nickname = watcher.WatcherNickname
	profile.Timezone = watcher.WatcherTimezone
	profile.AvatarURL = watcher.WatcherPicURL
	profile.CreateDatetime = watcher.CreateDatetime

	rows, err := db.Queryx("SELECT * FROM watcher_email WHERE watcher_id=? ORDER BY email_is_primary DESC, email_address", watcher.WatcherId)
	if err != nil {
		sublog.Fatal().Err(err).Str("table_name", "watcher_email").Msg("failed on SELECT")
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
