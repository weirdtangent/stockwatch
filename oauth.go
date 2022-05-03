package main

import (
	"database/sql"
	"errors"
	"time"
)

type OAuth struct {
	OAuthId        uint64    `db:"oauth_id"`
	OAuthIssuer    string    `db:"oauth_issuer"`
	OAuthSub       string    `db:"oauth_sub"`
	OAuthIssued    time.Time `db:"oauth_issued"`
	OAuthExpires   time.Time `db:"oauth_expires"`
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}

func (o *OAuth) create(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

	_, err := db.Exec(
		"INSERT INTO oauth SET oauth_issuer=?, oauth_sub=?, oauth_issued=?, oauth_expires=?",
		o.OAuthIssuer, o.OAuthSub, o.OAuthIssued, o.OAuthExpires)
	if err != nil {
		sublog.Error().Err(err).Str("table_name", "oauth").Msg("failed on insert")
		sublog.Debug().Interface("OAuth", o).Caller().Msg("failed on insert")
		return err
	}

	return o.getBySub(deps)
}

func (o *OAuth) createOrUpdate(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

	err := o.getBySub(deps)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return o.create(deps)
	}

	_, err = db.Exec(
		"UPDATE oauth SET oauth_issued=?, oauth_expires=?, update_datetime=now() WHERE oauth_id=?",
		o.OAuthIssued, o.OAuthExpires,
		o.OAuthId,
	)
	if err != nil {
		sublog.Error().Err(err).Str("table_name", "oauth").Msg("failed on update")
		sublog.Debug().Interface("OAuth", o).Caller().Msg("failed on update")
	}

	return o.getBySub(deps)
}

func (o *OAuth) getBySub(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM oauth WHERE oauth_sub=?", o.OAuthSub).StructScan(o)
	return err
}
