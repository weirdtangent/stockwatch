package main

import (
	"github.com/rs/zerolog/log"
)

func getExchange(acronym string) (*Exchange, error) {
	var exchange Exchange
	err := db_session.QueryRowx("SELECT * FROM exchange WHERE exchange_acronym = ?", acronym).StructScan(&exchange)
	return &exchange, err
}

func getExchangeById(exchange_id int64) (*Exchange, error) {
	var exchange Exchange
	err := db_session.QueryRowx("SELECT * FROM exchange WHERE exchange_id = ?", exchange_id).StructScan(&exchange)
	return &exchange, err
}

func createExchange(exchange *Exchange) (*Exchange, error) {
	var insert = "INSERT INTO exchange SET exchange_acronym=?, exchange_name=?"

	res, err := db_session.Exec(insert, exchange.Exchange_acronym, exchange.Exchange_name)
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
	return getExchangeById(exchange_id)
}

func getOrCreateExchange(exchange *Exchange) (*Exchange, error) {
	existing, err := getExchange(exchange.Exchange_acronym)
	if err != nil && existing.Exchange_id == 0 {
		return createExchange(exchange)
	}
	return existing, err
}

func createOrUpdateExchange(exchange *Exchange) (*Exchange, error) {
	var update = "UPDATE exchange SET exchange_name=?,country_id=?,city=? WHERE exchange_id=?"

	existing, err := getExchange(exchange.Exchange_acronym)
	if err != nil {
		return createExchange(exchange)
	}

	_, err = db_session.Exec(update, exchange.Exchange_name, exchange.Country_id, exchange.City, existing.Exchange_id)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "exchange").
			Msg("Failed on UPDATE")
	}
	return getExchangeById(existing.Exchange_id)
}
