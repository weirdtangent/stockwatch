package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func getExchange(db *sqlx.DB, acronym string) (*Exchange, error) {
	var exchange Exchange
	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_acronym = ?", acronym).StructScan(&exchange)
	return &exchange, err
}

func getExchangeById(db *sqlx.DB, exchange_id int64) (*Exchange, error) {
	var exchange Exchange
	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_id = ?", exchange_id).StructScan(&exchange)
	return &exchange, err
}

func createExchange(db *sqlx.DB, exchange *Exchange) (*Exchange, error) {
	var insert = "INSERT INTO exchange SET exchange_acronym=?, exchange_mic=?, exchange_name=?"

	res, err := db.Exec(insert, exchange.Exchange_acronym, exchange.Exchange_mic, exchange.Exchange_name)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "exchange").
			Msg("Failed on INSERT")
	}
	exchange_id, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "exchange").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getExchangeById(db, exchange_id)
}

func getOrCreateExchange(db *sqlx.DB, exchange *Exchange) (*Exchange, error) {
	existing, err := getExchange(db, exchange.Exchange_acronym)
	if err != nil && existing.Exchange_id == 0 {
		return createExchange(db, exchange)
	}
	return existing, err
}

func createOrUpdateExchange(db *sqlx.DB, exchange *Exchange) (*Exchange, error) {
	var update = "UPDATE exchange SET exchange_mic=?,exchange_name=?,country_id=?,city=? WHERE exchange_id=?"

	existing, err := getExchange(db, exchange.Exchange_acronym)
	if err != nil {
		return createExchange(db, exchange)
	}

	_, err = db.Exec(update, exchange.Exchange_mic, exchange.Exchange_name, exchange.Country_id, exchange.City, existing.Exchange_id)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "exchange").
			Msg("Failed on UPDATE")
	}
	return getExchangeById(db, existing.Exchange_id)
}
