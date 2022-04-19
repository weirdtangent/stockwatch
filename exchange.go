package main

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Exchange struct {
	ExchangeId      int64  `db:"exchange_id"`
	ExchangeMic     string `db:"exchange_mic"`
	OperatingMic    string `db:"operating_mic"`
	ExchangeName    string `db:"exchange_name"`
	ExchangeAcronym string `db:"exchange_acronym"`
	ExchangeCode    string `db:"exchange_code"`
	ExchangeTZ      string `db:"exchange_tz"`
	City            string `db:"city"`
	CountryId       int64  `db:"country_id"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

// object methods -------------------------------------------------------------

// misc -----------------------------------------------------------------------

func getExchangeById(ctx context.Context, exchange_id int64) (*Exchange, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var exchange Exchange
	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_id=?", exchange_id).StructScan(&exchange)
	return &exchange, err
}

func getExchangeByCode(ctx context.Context, exchangeCode string) (int64, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var exchangeId int64
	err := db.QueryRowx("SELECT exchange_id FROM exchange WHERE exchange_code=?", exchangeCode).Scan(&exchangeId)
	return exchangeId, err
}
