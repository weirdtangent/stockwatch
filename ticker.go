package stockwatch

import (
	"graystorm.com/mylog"
)

func getTicker(symbol string, exchange_id int64) (*Ticker, error) {
	var ticker Ticker
	err := db_session.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=? AND exchange_id=?", symbol, exchange_id).StructScan(&ticker)
	return &ticker, err
}

func getTickerFirstMatch(symbol string) (*Ticker, error) {
	var ticker Ticker
	err := db_session.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=? LIMIT 1", symbol).StructScan(&ticker)
	return &ticker, err
}

func getTickerById(ticker_id int64) (*Ticker, error) {
	var ticker Ticker
	err := db_session.QueryRowx("SELECT * FROM ticker WHERE ticker_id=?", ticker_id).StructScan(&ticker)
	return &ticker, err
}

func createTicker(ticker *Ticker) (*Ticker, error) {
	var insert = "INSERT INTO ticker SET ticker_symbol=?, exchange_id=?, ticker_name=?"

	res, err := db_session.Exec(insert, ticker.Ticker_symbol, ticker.Exchange_id, ticker.Ticker_name)
	if err != nil {
		mylog.Error.Fatal(err)
	}
	ticker_id, err := res.LastInsertId()
	if err != nil {
		mylog.Error.Fatal(err)
	}
	return getTickerById(ticker_id)
}

func getOrCreateTicker(ticker *Ticker) (*Ticker, error) {
	existing, err := getTicker(ticker.Ticker_symbol, ticker.Exchange_id)
	if err != nil && ticker.Ticker_id == 0 {
		return createTicker(ticker)
	}
	return existing, err
}

func createOrUpdateTicker(ticker *Ticker) (*Ticker, error) {
	var update = "UPDATE ticker SET exchange_id=?, ticker_name=? WHERE ticker_id=?"

	existing, err := getTicker(ticker.Ticker_symbol, ticker.Exchange_id)
	if err != nil {
		return createTicker(ticker)
	}

	_, err = db_session.Exec(update, ticker.Exchange_id, ticker.Ticker_name, existing.Ticker_id)
	if err != nil {
		mylog.Warning.Print(err)
	}
	return getTickerById(existing.Ticker_id)
}
