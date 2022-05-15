package main

import (
	"time"

	"github.com/rs/zerolog"
)

type Exchange struct {
	ExchangeId      uint64 `db:"exchange_id"`
	EId             string
	ExchangeMic     string    `db:"exchange_mic"`
	OperatingMic    string    `db:"operating_mic"`
	ExchangeName    string    `db:"exchange_name"`
	ExchangeAcronym string    `db:"exchange_acronym"`
	ExchangeCode    string    `db:"exchange_code"`
	ExchangeTZ      string    `db:"exchange_tz"`
	City            string    `db:"city"`
	CountryId       uint64    `db:"country_id"`
	CreateDatetime  time.Time `db:"create_datetime"`
	UpdateDatetime  time.Time `db:"update_datetime"`
}

// object methods -------------------------------------------------------------
func (e *Exchange) getByCode(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_code=?", e.ExchangeCode).StructScan(e)
	e.EId = encryptId(deps, *deps.logger, "exchange", e.ExchangeId)
	return err
}

// misc -----------------------------------------------------------------------

func getExchangeById(deps *Dependencies, sublog zerolog.Logger, exchange_id uint64) (Exchange, error) {
	db := deps.db

	exchange := Exchange{}
	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_id=?", exchange_id).StructScan(&exchange)
	if err == nil {
		exchange.EId = encryptId(deps, *deps.logger, "exchange", exchange.ExchangeId)
	}
	return exchange, err
}
