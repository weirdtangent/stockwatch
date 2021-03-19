package main

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Watcher struct {
	WatcherId      int64  `db:"watcher_id"`
	WatcherSub     string `db:"watcher_sub"`
	WatcherName    string `db:"watcher_name"`
	WatcherEmail   string `db:"watcher_email"`
	WatcherStatus  string `db:"watcher_status"`
	WatcherLevel   string `db:"watcher_level"`
	WatcherPicURL  string `db:"watcher_pic_url"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

func (w *Watcher) update(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	var update = "UPDATE watcher SET watcher_name=?, watcher_email=? WHERE watcher_id=?"
	_, err := db.Exec(update, w.WatcherName, w.WatcherEmail, w.WatcherId)
	if err != nil {
		log.Warn().Err(err).Str("table_name", "watcher").Msg("Failed on UPDATE")
	}
	return err
}

func (w *Watcher) create(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	var insert = "INSERT INTO watcher SET watcher_sub=?, watcher_name=?, watcher_email=?, watcher_status=?"
	res, err := db.Exec(insert, w.WatcherSub, w.WatcherName, w.WatcherEmail, w.WatcherStatus)
	if err != nil {
		log.Fatal().Err(err).Str("table_name", "watcher").Msg("Failed on INSERT")
		return err
	}
	watcherId, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).Str("table_name", "watcher").Msg("Failed on LAST_INSERT_ID")
		return err
	}
	w, err = getWatcherById(ctx, watcherId)
	return err
}

func (w *Watcher) getWatcherIdBySub(ctx context.Context) (int64, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var watcherId int64
	err := db.QueryRowx("SELECT watcher_id FROM watcher WHERE watcher_sub=?", w.WatcherSub).Scan(&watcherId)
	return watcherId, err
}

func (w *Watcher) createOrUpdate(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	watcherId, err := w.getWatcherIdBySub(ctx)
	if watcherId == 0 {
		return w.create(ctx)
	}
	w.WatcherId = watcherId

	var update = "UPDATE watcher SET watcher_name=?, watcher_email=?, watcher_status=?, watcher_pic_url=? WHERE watcher_sub=?"
	_, err = db.Exec(update, w.WatcherName, w.WatcherEmail, w.WatcherStatus, w.WatcherPicURL, w.WatcherSub)

	w, err = getWatcherBySub(ctx, w.WatcherSub)
	return err
}

func (w Watcher) IsAdmin() bool {
	return w.WatcherLevel == "admin" || w.WatcherLevel == "root"
}

func (w Watcher) IsRoot() bool {
	return w.WatcherLevel == "root"
}

// misc -----------------------------------------------------------------------
func getWatcherById(ctx context.Context, watcherId int64) (*Watcher, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var watcher Watcher
	err := db.QueryRowx("SELECT * FROM watcher WHERE watcher_id=?", watcherId).StructScan(&watcher)
	return &watcher, err
}

func getWatcherBySub(ctx context.Context, watcherSub string) (*Watcher, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var watcher Watcher
	err := db.QueryRowx("SELECT * FROM watcher WHERE watcher_sub=?", watcherSub).StructScan(&watcher)
	return &watcher, err
}
