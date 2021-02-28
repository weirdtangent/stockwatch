package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Watcher struct {
	WatcherId      int64  `db:"watcher_id"`
	WatcherName    string `db:"watcher_name"`
	WatcherEmail   string `db:"watcher_email"`
	OAuthId        int64  `db:"oauth_id"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

func (w Watcher) Update(db *sqlx.DB) error {
	var update = "UPDATE watcher SET watcher_name=?, watcher_email=?, oauth_id=? WHERE watcher_id=?"

	_, err := db.Exec(update, w.WatcherName, w.WatcherEmail, w.OAuthId, w.WatcherId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "watcher").
			Msg("Failed on UPDATE")
	}
	return err
}

func getWatcher(db *sqlx.DB, emailAddress string) (*Watcher, error) {
	var watcher Watcher
	err := db.QueryRowx("SELECT * FROM watcher WHERE watcher_email=?", emailAddress).StructScan(&watcher)
	return &watcher, err
}

func getWatcherById(db *sqlx.DB, watcherId int64) (*Watcher, error) {
	var watcher Watcher
	err := db.QueryRowx("SELECT * FROM watcher WHERE watcher_id=?", watcherId).StructScan(&watcher)
	return &watcher, err
}

func createWatcher(db *sqlx.DB, watcher *Watcher) (*Watcher, error) {
	var insert = "INSERT INTO watcher SET watcher_name=?, watcher_email=?, oauth_id=?"

	res, err := db.Exec(insert, watcher.WatcherName, watcher.WatcherEmail, watcher.OAuthId)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "watcher").
			Msg("Failed on INSERT")
	}
	watcherId, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "watcher").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getWatcherById(db, watcherId)
}

func getOrCreateWatcher(db *sqlx.DB, watcher *Watcher) (*Watcher, error) {
	existing, err := getWatcher(db, watcher.WatcherEmail)
	if err != nil && existing.WatcherId == 0 {
		return createWatcher(db, watcher)
	}
	return existing, err
}

func createOrUpdateWatcher(db *sqlx.DB, watcher *Watcher) (*Watcher, error) {
	var update = "UPDATE watcher SET watcher_name=?, watcher_email=?, oauth_id=? WHERE watcher_id=?"

	existing, err := getWatcher(db, watcher.WatcherEmail)
	if err != nil {
		return createWatcher(db, watcher)
	}

	_, err = db.Exec(update, watcher.WatcherName, watcher.WatcherEmail, watcher.OAuthId, existing.WatcherId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "watcher").
			Msg("Failed on UPDATE")
	}
	return getWatcherById(db, existing.WatcherId)
}
