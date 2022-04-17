package main

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type TickerSplit struct {
	TickerSplitId  int64  `db:"ticker_split_id"`
	TickerId       int64  `db:"ticker_id"`
	SplitDate      string `db:"split_date"`
	SplitRatio     string `db:"split_ratio"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

func (ts *TickerSplit) getByDate(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM ticker_split WHERE ticker_id=? AND split_date=?`, ts.TickerId, ts.SplitDate).StructScan(ts)
	return err
}

func (ts *TickerSplit) createIfNew(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if ts.SplitRatio == "" {
		// logger.Warn().Msg("Refusing to add ticker split with blank ratio")
		return nil
	}

	err := ts.getByDate(ctx)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_split SET ticker_id=?, split_date=?, split_ratio=?"
	_, err = db.Exec(insert, ts.TickerId, ts.SplitDate, ts.SplitRatio)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker_split").
			Msg("Failed on INSERT")
	}
	return err
}
