package main

import "time"

type Exchange struct {
	ExchangeId      uint64    `db:"exchange_id"`
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

func (e *Exchange) getById(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_id=?", e.ExchangeId).StructScan(e)
	return err
}

func (e *Exchange) getByCode(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_code=?", e.ExchangeCode).StructScan(e)
	return err
}

// misc -----------------------------------------------------------------------
