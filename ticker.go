package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func getTicker(db *sqlx.DB, symbol string, exchange_id int64) (*Ticker, error) {
	var ticker Ticker
	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=? AND exchange_id=?", symbol, exchange_id).StructScan(&ticker)
	return &ticker, err
}

func getTickerFirstMatch(db *sqlx.DB, symbol string) (*Ticker, error) {
	var ticker Ticker
	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=? LIMIT 1", symbol).StructScan(&ticker)
	return &ticker, err
}

func getTickerById(db *sqlx.DB, tickerId int64) (*Ticker, error) {
	var ticker Ticker
	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_id=?", tickerId).StructScan(&ticker)
	return &ticker, err
}

func createTicker(db *sqlx.DB, ticker *Ticker) (*Ticker, error) {
	var insert = "INSERT INTO ticker SET ticker_symbol=?, exchange_id=?, ticker_name=?"

	res, err := db.Exec(insert, ticker.TickerSymbol, ticker.ExchangeId, ticker.TickerName)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "ticker").
			Str("symbol", ticker.TickerSymbol).
			Int64("tickerId", ticker.TickerId).
			Msg("Failed on INSERT")
	}
	tickerId, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "ticker").
			Str("symbol", ticker.TickerSymbol).
			Int64("tickerId", ticker.TickerId).
			Msg("Failed on LAST_INSERTId")
	}
	return getTickerById(db, tickerId)
}

func getOrCreateTicker(db *sqlx.DB, ticker *Ticker) (*Ticker, error) {
	existing, err := getTicker(db, ticker.TickerSymbol, ticker.ExchangeId)
	if err != nil && ticker.TickerId == 0 {
		return createTicker(db, ticker)
	}
	return existing, err
}

func createOrUpdateTicker(db *sqlx.DB, ticker *Ticker) (*Ticker, error) {
	var update = "UPDATE ticker SET exchange_id=?, ticker_name=? WHERE ticker_id=?"

	existing, err := getTicker(db, ticker.TickerSymbol, ticker.ExchangeId)
	if err != nil {
		return createTicker(db, ticker)
	}

	_, err = db.Exec(update, ticker.ExchangeId, ticker.TickerName, existing.TickerId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "ticker").
			Str("symbol", ticker.TickerSymbol).
			Int64("tickerId", ticker.TickerId).
			Msg("Failed on UPDATE")
	}
	return getTickerById(db, existing.TickerId)
}
