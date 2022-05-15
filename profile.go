package main

import (
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
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

// object methods -------------------------------------------------------------

// misc -----------------------------------------------------------------------

func profileHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata

		watcher := checkAuthState(w, r, deps, *deps.logger)
		if watcher.WatcherId == 0 {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		sublog := deps.logger.With().Str("watcher", watcher.EId).Logger()

		params := mux.Vars(r)
		status := params["status"]

		if status == "welcome" {
			webdata["announcement"] = []string{`
			Welcome to Stockwatch! This is really just a hobby site for me to learn Go programming, but I
			encourage feedback to let me know cool stuff I should try! I need to work on a feedback form,
			but meanwhile you can just email request@graystorm.com. I suspect, though, I will most often be
			thinking, "yeah, I dunno how to do that" ;) Also, these profile settings below don't quite
			work yet, but I'm working on it!`}
			updateWatcher(deps, sublog, watcher)
		}

		profile, err := getProfile(deps, sublog, watcher)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to get profile info")
		}
		webdata["profile"] = profile

		timezones := getTimezones(deps, sublog)

		sort.Slice(timezones, func(i, j int) bool {
			return timezones[i].Location < timezones[j].Location
		})

		webdata["timezones"] = timezones

		renderTemplate(w, r, deps, sublog, "profile")
	})
}

func getProfile(deps *Dependencies, sublog zerolog.Logger, watcher Watcher) (*Profile, error) {
	db := deps.db

	profile := Profile{}

	profile.Name = watcher.WatcherName
	profile.Nickname = watcher.WatcherNickname
	profile.Timezone = watcher.WatcherTimezone
	profile.AvatarURL = watcher.WatcherPicURL
	profile.CreateDatetime = watcher.CreateDatetime

	rows, err := db.Queryx("SELECT * FROM watcher_email WHERE watcher_id=? ORDER BY email_is_primary DESC, email_address", watcher.WatcherId)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed on SELECT")
	}
	defer rows.Close()

	watcherEmail := WatcherEmail{}
	emails := make([]ProfileEmail, 0)
	for rows.Next() {
		err = rows.StructScan(&watcherEmail)
		if err != nil {
			sublog.Fatal().Err(err).Msg("failed reading result rows")
		}
		emails = append(emails, ProfileEmail{watcherEmail.EmailAddress, watcherEmail.IsPrimary})
	}
	if err := rows.Err(); err != nil {
		sublog.Fatal().Err(err).Msg("failed reading result rows")
	}

	profile.Emails = emails
	return &profile, nil
}
