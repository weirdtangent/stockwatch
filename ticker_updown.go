package main

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type TickerUpDown struct {
	TickerUpDownId  int64  `db:"updown_id"`
	TickerId        int64  `db:"ticker_id"`
	UpDownAction    string `db:"updown_action"`
	UpDownFromGrade string `db:"updown_fromgrade"`
	UpDownToGrade   string `db:"updown_tograde"`
	UpDownDate      string `db:"updown_date"`
	UpDownFirm      string `db:"updown_firm"`
	UpDownSince     string `db:"updown_since"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

func (tud *TickerUpDown) getByUniqueKey(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM ticker_updown WHERE ticker_id=? AND updown_date=? AND updown_firm=?`, tud.TickerId, tud.UpDownDate, tud.UpDownFirm).StructScan(tud)
	return err
}

func (tud *TickerUpDown) createIfNew(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if tud.UpDownToGrade == "" {
		return nil
	}

	// if already exists, just quietly return
	err := tud.getByUniqueKey(ctx)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_updown SET ticker_id=?, updown_action=?, updown_fromgrade=?, updown_tograde=?, updown_date=?, updown_firm=?"
	_, err = db.Exec(insert, tud.TickerId, tud.UpDownAction, tud.UpDownFromGrade, tud.UpDownToGrade, tud.UpDownDate, tud.UpDownFirm)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker_updown").
			Msg("Failed on INSERT")
	}
	return err
}
