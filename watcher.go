package main

import (
	"database/sql"
	"errors"

	"github.com/rs/zerolog/log"
)

type Watcher struct {
	WatcherId       uint64       `db:"watcher_id"`
	WatcherSub      string       `db:"watcher_sub"`
	WatcherName     string       `db:"watcher_name"`
	WatcherNickname string       `db:"watcher_nickname"`
	WatcherStatus   string       `db:"watcher_status"`
	WatcherLevel    string       `db:"watcher_level"`
	WatcherTimezone string       `db:"watcher_timezone"`
	WatcherPicURL   string       `db:"watcher_pic_url"`
	SessionId       string       `db:"session_id"`
	CreateDatetime  sql.NullTime `db:"create_datetime"`
	UpdateDatetime  sql.NullTime `db:"update_datetime"`
}

type WatcherEmail struct {
	WatcherEmailId uint64       `db:"watcher_email_id"`
	WatcherId      uint64       `db:"watcher_id"`
	EmailAddress   string       `db:"email_address"`
	IsPrimary      bool         `db:"email_is_primary"`
	CreateDatetime sql.NullTime `db:"create_datetime"`
	UpdateDatetime sql.NullTime `db:"update_datetime"`
}

type WatcherRecent struct {
	WatcherRecentId uint64       `db:"watcher_recent_id"`
	WatcherId       uint64       `db:"watcher_id"`
	TickerId        uint64       `db:"ticker_id"`
	TickerSymbol    string       `db:"ticker_symbol"`
	Locked          bool         `db:"locked"`
	CreateDatetime  sql.NullTime `db:"create_datetime"`
	UpdateDatetime  sql.NullTime `db:"update_datetime"`
}

type WebWatcher struct {
	WatcherNickname string
	WatcherStatus   string
	WatcherLevel    string
	WatcherTimezone string
	WatcherPicURL   string
}

func getWatcherById(deps *Dependencies, watcherId uint64) (Watcher, error) {
	db := deps.db

	w := Watcher{}
	err := db.QueryRowx("SELECT * FROM watcher WHERE watcher_id=?", watcherId).StructScan(&w)
	return w, err
}

func updateWatcher(deps *Dependencies, w Watcher) error {
	db := deps.db

	update := "UPDATE watcher SET watcher_name=?, watcher_nickname=?, update_createtime=now() WHERE watcher_id=?"
	_, err := db.Exec(update, w.WatcherName, w.WatcherNickname, w.WatcherId)
	return err
}

func updateWatcherFromOAuth(deps *Dependencies, w Watcher, email string) error {
	db := deps.db

	update := "UPDATE watcher SET watcher_name=?, watcher_pic_url=?, session_id=? WHERE watcher_id=?"
	_, err := db.Exec(update, w.WatcherName, w.WatcherPicURL, w.SessionId, w.WatcherId)
	if err != nil {
		return err
	}

	update = "INSERT INTO watcher_email SET watcher_id=?, email_address=? ON DUPLICATE KEY UPDATE watcher_id=watcher_id"
	_, err = db.Exec(update, w.WatcherId, email)
	return err
}

func createWatcher(deps *Dependencies, w Watcher, email string) (Watcher, error) {
	db := deps.db

	insert := "INSERT INTO watcher SET watcher_sub=?, watcher_name=?, watcher_nickname=?, watcher_status=?, watcher_pic_url=?, session_id=?"
	_, err := db.Exec(insert, w.WatcherSub, w.WatcherName, w.WatcherNickname, w.WatcherStatus, w.WatcherPicURL, w.SessionId)
	if err != nil {
		return Watcher{}, err
	}

	w.WatcherId, err = getWatcherIdBySession(deps, w.SessionId)
	if err != nil {
		return Watcher{}, err
	}

	insert = "INSERT INTO watcher_email SET watcher_id=?, email_address=?, email_is_primary=1"
	_, err = db.Exec(insert, w.WatcherId, email)

	return w, err
}

func createOrUpdateWatcherFromOAuth(deps *Dependencies, watcher Watcher, email string) (Watcher, error) {
	watcherId, err := getWatcherIdBySession(deps, watcher.SessionId)
	if err != nil {
		watcher.WatcherId = watcherId
		err := updateWatcherFromOAuth(deps, watcher, email)
		return watcher, err
	}

	if watcherId == 0 {
		watcherId, err = getWatcherIdByEmail(deps, email)
		if err != nil {
			watcher.WatcherId = watcherId
			err := updateWatcherFromOAuth(deps, watcher, email)
			return watcher, err
		}
		if watcherId == 0 {
			return createWatcher(deps, watcher, email)
		}
	}

	watcher.WatcherId = watcherId
	err = updateWatcherFromOAuth(deps, watcher, email)
	return watcher, err
}

func (w Watcher) IsAdmin() bool {
	return w.WatcherLevel == "admin" || w.WatcherLevel == "root"
}

func (w Watcher) IsRoot() bool {
	return w.WatcherLevel == "root"
}

// misc -----------------------------------------------------------------------
func getWatcherBySession(deps *Dependencies, session string) (Watcher, error) {
	db := deps.db

	w := Watcher{}
	err := db.QueryRowx("SELECT * FROM watcher WHERE session_id=?", session).StructScan(&w)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return Watcher{}, nil
	}
	return w, err
}

func isNicknameAvailable(deps *Dependencies, watcherId uint64, nickname string) bool {
	db := deps.db

	var count int
	err := db.QueryRowx("SELECT count(*) FROM watcher WHERE watcher_id != ? and watcher_nickname=?", watcherId, nickname).Scan(&count)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return true
	}
	return false
}

func getWatcherIdBySession(deps *Dependencies, session string) (uint64, error) {
	db := deps.db
	sublog := deps.logger

	var watcherId uint64
	err := db.QueryRowx("SELECT watcher_id FROM watcher WHERE session_id=?", session).Scan(&watcherId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		} else {
			sublog.Warn().Err(err).Str("table_name", "watcher").Msg("Failed to check for existing record")
		}
	}
	return watcherId, err
}

func getWatcherIdByEmail(deps *Dependencies, email string) (uint64, error) {
	db := deps.db
	sublog := deps.logger

	var watcherId uint64
	err := db.QueryRowx("SELECT watcher_id FROM watcher_email WHERE email_address=?", email).Scan(&watcherId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		} else {
			sublog.Warn().Err(err).Str("table_name", "watcher").Msg("Failed to check for existing record")
		}
	}
	return watcherId, err
}

func (wr *WatcherRecent) create(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

	insert := "INSERT INTO watcher_recent SET watcher_id=?, ticker_id=?, locked=?"
	_, err := db.Exec(insert, wr.WatcherId, wr.TickerId, wr.Locked)
	if err != nil {
		sublog.Error().Err(err).Str("table_name", "watcher_recent").Msg("failed on INSERT")
		return err
	}

	return nil
}

func (wr *WatcherRecent) createOrUpdate(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

	insert_or_update := "INSERT INTO watcher_recent SET watcher_id=?, ticker_id=?, locked=? ON DUPLICATE KEY UPDATE locked=?, update_datetime=NOW()"
	_, err := db.Exec(insert_or_update, wr.WatcherId, wr.TickerId, wr.Locked, wr.Locked)
	if err != nil {
		sublog.Error().Err(err).Str("table_name", "watcher_recent").Msg("failed on INSERT OR UPDATE")
		return err
	}

	return nil
}

func lockWatcherRecent(deps *Dependencies, watcher Watcher, ticker Ticker) bool {
	db := deps.db

	var update = "UPDATE watcher_recent SET locked=true WHERE watcher_id=? AND ticker_id=?"
	_, err := db.Exec(update, watcher.WatcherId, ticker.TickerId)
	if err != nil {
		log.Warn().Err(err).Str("table_name", "watcher_recent").Msg("Failed on UPDATE")
		return false
	}
	return true
}

func unlockWatcherRecent(deps *Dependencies, watcher Watcher, ticker Ticker) bool {
	db := deps.db

	var update = "UPDATE watcher_recent SET locked=false WHERE watcher_id=? AND ticker_id=?"
	_, err := db.Exec(update, watcher.WatcherId, ticker.TickerId)
	if err != nil {
		log.Warn().Err(err).Str("table_name", "watcher_recent").Msg("Failed on UPDATE")
		return false
	}
	return true
}
