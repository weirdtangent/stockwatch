package main

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type Exchange struct {
	ExchangeId      uint64       `db:"exchange_id"`
	ExchangeMic     string       `db:"exchange_mic"`
	OperatingMic    string       `db:"operating_mic"`
	ExchangeName    string       `db:"exchange_name"`
	ExchangeAcronym string       `db:"exchange_acronym"`
	ExchangeCode    string       `db:"exchange_code"`
	ExchangeTZ      string       `db:"exchange_tz"`
	City            string       `db:"city"`
	CountryId       uint64       `db:"country_id"`
	CreateDatetime  sql.NullTime `db:"create_datetime"`
	UpdateDatetime  sql.NullTime `db:"update_datetime"`
}

// object methods -------------------------------------------------------------

func (e *Exchange) getById(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_id=?", e.ExchangeId).StructScan(e)
	return err
}

func (e *Exchange) getByCode(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_code=?", e.ExchangeCode).StructScan(e)
	return err
}

// misc -----------------------------------------------------------------------
