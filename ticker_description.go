package main

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type TickerDescription struct {
	TickerDescriptionId int64  `db:"description_id"`
	TickerId            int64  `db:"ticker_id"`
	BusinessSummary     string `db:"business_summary"`
	CreateDatetime      string `db:"create_datetime"`
	UpdateDatetime      string `db:"update_datetime"`
}

func (td *TickerDescription) getByUniqueKey(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM ticker_description WHERE ticker_id=?`, td.TickerId).StructScan(td)
	return err
}

func (td *TickerDescription) createIfNew(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if td.BusinessSummary == "" {
		return nil
	}

	err := td.getByUniqueKey(ctx)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_description SET ticker_id=?, business_summary=?"
	_, err = db.Exec(insert, td.TickerId, td.BusinessSummary)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker_description").
			Msg("Failed on INSERT")
	}
	return err
}

// misc -----------------------------------------------------------------------

func getTickerDescriptionByTickerId(ctx context.Context, ticker_id int64) (*TickerDescription, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var tickerDescription TickerDescription
	err := db.QueryRowx(`SELECT * FROM ticker_description WHERE ticker_id=?`, ticker_id).StructScan(&tickerDescription)
	return &tickerDescription, err
}
