package main

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type OAuth struct {
	OAuthId        uint64       `db:"oauth_id"`
	OAuthIssuer    string       `db:"oauth_issuer"`
	OAuthSub       string       `db:"oauth_sub"`
	OAuthIssued    sql.NullTime `db:"oauth_issued"`
	OAuthExpires   sql.NullTime `db:"oauth_expires"`
	CreateDatetime sql.NullTime `db:"create_datetime"`
	UpdateDatetime sql.NullTime `db:"update_datetime"`
}

func (o *OAuth) create(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	_, err := db.Exec(
		"INSERT INTO oauth SET oauth_issuer=?, oauth_sub=?, oauth_issued=?, oauth_expires=?",
		o.OAuthIssuer, o.OAuthSub, o.OAuthIssued, o.OAuthExpires)
	if err != nil {
		log.Error().Err(err).Str("table_name", "oauth").Msg("failed on insert")
		log.Debug().Interface("OAuth", o).Caller().Msg("failed on insert")
		return err
	}

	return o.getBySub(ctx)
}

func (o *OAuth) createOrUpdate(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := o.getBySub(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return o.create(ctx)
	}

	_, err = db.Exec(
		"UPDATE oauth SET oauth_issued=?, oauth_expires=? WHERE oauth_id=?",
		o.OAuthIssued, o.OAuthExpires,
		o.OAuthId,
	)
	if err != nil {
		log.Error().Err(err).Str("table_name", "oauth").Msg("failed on update")
		log.Debug().Interface("OAuth", o).Caller().Msg("failed on update")
	}

	return o.getBySub(ctx)
}

func (o *OAuth) getBySub(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM oauth WHERE oauth_sub=?", o.OAuthSub).StructScan(o)
	return err
}
