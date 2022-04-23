package main

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Watcher struct {
	WatcherId       int64  `db:"watcher_id"`
	WatcherSub      string `db:"watcher_sub"`
	WatcherName     string `db:"watcher_name"`
	WatcherStatus   string `db:"watcher_status"`
	WatcherLevel    string `db:"watcher_level"`
	WatcherTimezone string `db:"watcher_timezone"`
	WatcherPicURL   string `db:"watcher_pic_url"`
	SessionId       string `db:"session_id"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

type WatcherEmail struct {
	WatcherEmailId int64  `db:"watcher_email_id"`
	WatcherId      int64  `db:"watcher_id"`
	EmailAddress   string `db:"email_address"`
	IsPrimary      bool   `db:"email_is_primary"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

func (w *Watcher) update(ctx context.Context, email string) error {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var update = "UPDATE watcher SET watcher_name=?, watcher_pic_url=?, session_id=? WHERE watcher_id=?"
	_, err := db.Exec(update, w.WatcherName, w.WatcherPicURL, w.SessionId, w.WatcherId)
	if err != nil {
		logger.Warn().Err(err).Str("table_name", "watcher").Msg("Failed on UPDATE")
	} else {
		err = getWatcherById(ctx, w, w.WatcherId)
		if err != nil {
			logger.Warn().Err(err).Int64("watcher_id", w.WatcherId).Str("table_name", "watcher").Msg("Failed to retrieve record after UPDATE")
		}
	}

	if err == nil {
		var update = "INSERT INTO watcher_email SET watcher_id=?, email_address=? ON DUPLICATE KEY UPDATE watcher_id=watcher_id"
		_, err = db.Exec(update, w.WatcherId, email)
		if err != nil {
			logger.Warn().Err(err).Str("table_name", "watcher_email").Msg("Failed to store/ignore email address after UPDATE")
		}
	}
	return err
}

func (w *Watcher) create(ctx context.Context, email string) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	insert := "INSERT INTO watcher SET watcher_sub=?, watcher_name=?, watcher_status=?, watcher_pic_url=?, session_id=?"
	_, err := db.Exec(insert, w.WatcherSub, w.WatcherName, w.WatcherStatus, w.WatcherPicURL, w.SessionId)
	if err != nil {
		log.Error().Err(err).Str("table_name", "watcher").Msg("failed on INSERT")
		return err
	}
	w.WatcherId, err = getWatcherIdBySession(ctx, w.SessionId)
	if err != nil || w.WatcherId == 0 {
		log.Error().Err(err).Str("table_name", "watcher").Msg("failed on getting watcher_id of who we just inserted")
		return err
	}
	insert = "INSERT INTO watcher_email SET watcher_id=?, email_address=?, email_is_primary=1"
	_, err = db.Exec(insert, w.WatcherId, email)
	if err != nil {
		log.Warn().Err(err).Str("table_name", "watcher_email").Msg("failed on INSERT")
	}

	return nil
}

func (w *Watcher) createOrUpdate(ctx context.Context, email string) error {
	watcherId, err := getWatcherIdBySession(ctx, w.SessionId)
	if err != nil {
		return nil
	}

	if watcherId == 0 {
		log.Info().Msg("did not connect to watcher via sessionId")
		watcherId, err = getWatcherIdByEmail(ctx, email)
		if err != nil {
			return nil
		}
		if watcherId == 0 {
			log.Info().Msg("did not connect to watcher via emailAddress")
			return w.create(ctx, email)
		}
	}

	w.WatcherId = watcherId

	return w.update(ctx, email)
}

func (w Watcher) IsAdmin() bool {
	return w.WatcherLevel == "admin" || w.WatcherLevel == "root"
}

func (w Watcher) IsRoot() bool {
	return w.WatcherLevel == "root"
}

// misc -----------------------------------------------------------------------
func getWatcherById(ctx context.Context, w *Watcher, watcherId int64) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM watcher WHERE watcher_id=?", watcherId).StructScan(w)
	return err
}

func getWatcherIdBySession(ctx context.Context, session string) (int64, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var watcherId int64
	err := db.QueryRowx("SELECT watcher_id FROM watcher WHERE session_id=?", session).Scan(&watcherId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		} else {
			logger.Warn().Err(err).Str("table_name", "watcher").Msg("Failed to check for existing record")
		}
	}
	return watcherId, err
}

func getWatcherIdByEmail(ctx context.Context, email string) (int64, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var watcherId int64
	err := db.QueryRowx("SELECT watcher_id FROM watcher_email WHERE email_address=?", email).Scan(&watcherId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		} else {
			logger.Warn().Err(err).Str("table_name", "watcher").Msg("Failed to check for existing record")
		}
	}
	return watcherId, err
}
