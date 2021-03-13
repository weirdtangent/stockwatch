package main

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Exchange struct {
	ExchangeId      int64  `db:"exchange_id"`
	ExchangeAcronym string `db:"exchange_acronym"`
	ExchangeMic     string `db:"exchange_mic"`
	ExchangeName    string `db:"exchange_name"`
	CountryId       int64  `db:"country_id"`
	City            string `db:"city"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

// default search is by ACRONYM or MIC
func getExchange(db *sqlx.DB, acronym string, mic string) (*Exchange, error) {
	var exchange Exchange
	var err error
	// if either of these come in as blank searches, make them not match anything
	if acronym != "" {
		err = db.QueryRowx("SELECT * FROM exchange WHERE exchange_acronym = ?", acronym).StructScan(&exchange)
	} else if mic != "" {
		err = db.QueryRowx("SELECT * FROM exchange WHERE exchange_mic = ?", mic).StructScan(&exchange)
	} else {
		return nil, fmt.Errorf("Acronym or Mic must not be blank to perform search")
	}
	return &exchange, err
}

func getExchangeById(db *sqlx.DB, exchange_id int64) (*Exchange, error) {
	var exchange Exchange
	err := db.QueryRowx("SELECT * FROM exchange WHERE exchange_id = ?", exchange_id).StructScan(&exchange)
	return &exchange, err
}

func createExchange(db *sqlx.DB, exchange *Exchange) (*Exchange, error) {
	var insert = "INSERT INTO exchange SET exchange_acronym=?, exchange_mic=?, exchange_name=?"

	if exchange.ExchangeAcronym == "" {
		return exchange, fmt.Errorf("Skipping record with blank exchange acronym")
	}

	res, err := db.Exec(insert, exchange.ExchangeAcronym, exchange.ExchangeMic, exchange.ExchangeName)
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
	existing, err := getExchange(db, exchange.ExchangeAcronym, exchange.ExchangeMic)
	if err != nil && existing.ExchangeId == 0 {
		return createExchange(db, exchange)
	}
	return existing, err
}

func createOrUpdateExchange(db *sqlx.DB, exchange *Exchange) (*Exchange, error) {
	var update = "UPDATE exchange SET exchange_mic=?,exchange_name=?,country_id=?,city=? WHERE exchange_id=?"

	if exchange.ExchangeAcronym == "" && exchange.ExchangeMic == "" {
		return exchange, fmt.Errorf("Skipping record with blank exchange acronym and mic")
	}

	existing, err := getExchange(db, exchange.ExchangeAcronym, exchange.ExchangeMic)
	if err != nil {
		return createExchange(db, exchange)
	}

	_, err = db.Exec(update, exchange.ExchangeMic, exchange.ExchangeName, exchange.CountryId, exchange.City, existing.ExchangeId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "exchange").
			Msg("Failed on UPDATE")
	}
	return getExchangeById(db, existing.ExchangeId)
}
