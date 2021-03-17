package main

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Exchange struct {
	ExchangeId      int64  `db:"exchange_id"`
	ExchangeAcronym string `db:"exchange_acronym"`
	ExchangeName    string `db:"exchange_name"`
	ExchangeMic     string `db:"exchange_mic"`
	CountryId       int64  `db:"country_id"`
	City            string `db:"city"`
	ExchangeTZ      string `db:"exchange_tz"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

// object methods -------------------------------------------------------------

func (e *Exchange) getByUniqueKey(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_acronym = ?", e.ExchangeAcronym).StructScan(e)
	return err
}

func (e *Exchange) getOrCreate(ctx context.Context) error {
	err := e.getByUniqueKey(ctx)
	if err != nil && e.ExchangeId == 0 {
		return e.create(ctx)
	}
	return err
}

func (e *Exchange) create(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	if e.ExchangeAcronym == "" {
		return fmt.Errorf("Refusing to add exchange with blank acronym")
	}

	var insert = "INSERT INTO exchange SET exchange_acronym=?, exchange_name=?, exchange_mic=?, exchange_tz=?"
	res, err := db.Exec(insert, e.ExchangeAcronym, e.ExchangeName, e.ExchangeMic, e.ExchangeTZ)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "exchange").
			Msg("Failed on INSERT")
		return err
	}
	e.ExchangeId, err = res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "exchange").
			Msg("Failed on LAST_INSERT_ID")
		return err
	}
	return err
}

// misc -----------------------------------------------------------------------

func getExchangeById(ctx context.Context, exchange_id int64) (*Exchange, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var exchange Exchange
	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_id=?", exchange_id).StructScan(&exchange)
	return &exchange, err
}
