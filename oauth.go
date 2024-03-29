package main

import (
	"database/sql"
	"errors"
	"time"

	"github.com/rs/zerolog"
)

type OAuth struct {
	OAuthId        uint64 `db:"oauth_id"`
	EId            string
	OAuthIssuer    string    `db:"oauth_issuer"`
	OAuthSub       string    `db:"oauth_sub"`
	OAuthIssued    time.Time `db:"oauth_issued"`
	OAuthExpires   time.Time `db:"oauth_expires"`
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}

// object methods -------------------------------------------------------------

func (o *OAuth) create(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger.With().Str("provider", o.OAuthIssuer).Logger()

	_, err := db.Exec(
		"INSERT INTO oauth SET oauth_issuer=?, oauth_sub=?, oauth_issued=?, oauth_expires=?",
		o.OAuthIssuer, o.OAuthSub, o.OAuthIssued, o.OAuthExpires)
	if err != nil {
		sublog.Error().Err(err).Msg("failed on insert")
		return err
	}

	return o.getBySub(deps)
}

func (o *OAuth) createOrUpdate(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db
	sublog = sublog.With().Str("provider", o.OAuthIssuer).Logger()

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
		sublog.Error().Err(err).Msg("failed on update")
	}

	return o.getBySub(deps)
}

func (o *OAuth) getBySub(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM oauth WHERE oauth_sub=?", o.OAuthSub).StructScan(o)
	return err
}

// misc -----------------------------------------------------------------------
