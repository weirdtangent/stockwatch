package main

import (
	"graystorm.com/mylog"
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
		mylog.Error.Fatal(err)
	}
	exchange_id, err := res.LastInsertId()
	if err != nil {
		mylog.Error.Fatal(err)
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
	var update = "UPDATE exchange SET exchange_name=?,country_id=? WHERE exchange_id=?"

	existing, err := getExchange(exchange.Exchange_acronym)
	if err != nil {
		return createExchange(exchange)
	}

	_, err = db_session.Exec(update, exchange.Exchange_name, exchange.Country_id, existing.Exchange_id)
	if err != nil {
		mylog.Warning.Print(err)
	}
	return getExchangeById(existing.Exchange_id)
}
