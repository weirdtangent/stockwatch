package main

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type OAuth struct {
	OAuthId        int64  `db:"oauth_id"`
	OAuthIssuer    string `db:"oauth_issuer"`
	OAuthSub       string `db:"oauth_sub"`
	OAuthIssued    int64  `db:"oauth_issued"`
	OAuthExpires   int64  `db:"oauth_expires"`
	SessionId      string `db:"session_id"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

func (o *OAuth) setStatus(ctx context.Context, newStatus string) error {
	db := ctx.Value("db").(*sqlx.DB)

	_, err := db.Exec("UPDATE oauth SET oauth_status=? WHERE oauth_id=?", newStatus, o.OAuthId)
	return err
}

func (o *OAuth) checkBySubscriber(ctx context.Context) int64 {
	db := ctx.Value("db").(*sqlx.DB)

	var oauthId int64
	db.QueryRowx("SELECT oauth_id FROM oauth WHERE oauth_sub=?", o.OAuthSub).Scan(&oauthId)
	return oauthId
}

func (o *OAuth) create(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)
	logger := log.Ctx(ctx)

	var insert = "INSERT INTO oauth SET oauth_issuer=?, oauth_sub=?, oauth_issued=?, oauth_expires=?, session_id=?"
	_, err := db.Exec(insert, o.OAuthIssuer, o.OAuthSub, o.OAuthIssued, o.OAuthExpires, o.SessionId)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "oauth").
			Msg("Failed on INSERT")
		return err
	}

	o, err = getOAuthBySub(ctx, o.OAuthSub)
	return err
}

func (o *OAuth) createOrUpdate(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)
	logger := log.Ctx(ctx)

	o.OAuthId = o.checkBySubscriber(ctx)
	if o.OAuthId == 0 {
		return o.create(ctx)
	}

	var update = "UPDATE oauth SET oauth_issued=?, oauth_expires=?, session_id=? WHERE oauth_id=?"
	_, err := db.Exec(update, o.OAuthIssued, o.OAuthExpires, o.SessionId, o.OAuthId)
	if err != nil {
		logger.Warn().Err(err).
			Str("table_name", "oauth").
			Msg("Failed on UPDATE")
	}

	o, err = getOAuthBySub(ctx, o.OAuthSub)
	return err
}

func getOAuthBySub(ctx context.Context, sub string) (*OAuth, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var oauth OAuth
	err := db.QueryRowx("SELECT * FROM oauth WHERE oauth_sub=?", sub).StructScan(&oauth)
	return &oauth, err
}
