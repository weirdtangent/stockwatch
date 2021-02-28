package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/mytime"
)

type Ticker struct {
	TickerId       int64  `db:"ticker_id"`
	TickerSymbol   string `db:"ticker_symbol"`
	ExchangeId     int64  `db:"exchange_id"`
	TickerName     string `db:"ticker_name"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

func (t Ticker) LoadDailies(db *sqlx.DB, days int) ([]Daily, error) {
	var daily Daily
	dailies := make([]Daily, 0, days)

	rows, err := db.Queryx(
		`SELECT * FROM (
       SELECT * FROM daily WHERE ticker_id=? AND volume > 0
		   ORDER BY price_date DESC LIMIT ?) DT1
		 ORDER BY price_date`,
		t.TickerId, days)
	if err != nil {
		return dailies, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&daily)
		if err != nil {
			log.Warn().Err(err).
				Str("table_name", "daily").
				Msg("Error reading result rows")
		} else {
			dailies = append(dailies, daily)
		}
	}
	if err := rows.Err(); err != nil {
		return dailies, err
	}

	return dailies, nil
}

func (t Ticker) LoadIntraday(db *sqlx.DB, intradate string) ([]Intraday, error) {
	var intraday Intraday
	intradays := make([]Intraday, 0)

	rows, err := db.Queryx(
		`SELECT * FROM intraday                                                                                                                                                  
		 WHERE ticker_id=? AND price_date LIKE ? AND volume > 0                                                                                                                  
		 ORDER BY price_date`,
		t.TickerId, intradate+"%")
	if err != nil {
		return intradays, err
	}
	defer rows.Close()

	// add pre-closing price
	priorBusinessDay, err := mytime.PriorBusinessDayStr(intradate + " 21:05:00")
	if err != nil {
		return intradays, fmt.Errorf("Failed to get prior business day date")
	}
	preDaily, err := getDaily(db, t.TickerId, priorBusinessDay)
	if err == nil {
		intradays = append(intradays, Intraday{0, t.TickerId, priorBusinessDay, preDaily.ClosePrice, 0, "", ""})
	} else {
		log.Info().Msg("PriorBusinessDay close price was NOT included")
	}

	// add these intraday prices
	for rows.Next() {
		err = rows.StructScan(&intraday)
		if err != nil {
			log.Warn().Err(err).
				Str("table_name", "intraday").
				Msg("Error reading result rows")
		} else {
			intradays = append(intradays, intraday)
		}
	}
	if err := rows.Err(); err != nil {
		return intradays, err
	}

	// add post-opening price
	nextBusinessDay, err := mytime.NextBusinessDayStr(intradate + " 13:55:00")
	if err != nil {
		return intradays, fmt.Errorf("Failed to get next business day date")
	}
	postDaily, err := getDaily(db, t.TickerId, nextBusinessDay)
	if err == nil {
		intradays = append(intradays, Intraday{0, t.TickerId, nextBusinessDay, postDaily.OpenPrice, 0, "", ""})
	} else {
		log.Info().Msg("NextBusinessDay open price was NOT included")
	}

	return intradays, nil
}

// see if we need to pull a daily update:
//  if we don't have the EOD price for the prior business day
//  OR if we don't have it for the current business day and it's now 7pm or later
func (t Ticker) updateDailies(awssess *session.Session, db *sqlx.DB) (bool, error) {
	mostRecentDaily, err := getDailyMostRecent(db, t.TickerId)
	if err != nil {
		return false, err
	}
	mostRecentDailyDate := mostRecentDaily.PriceDate
	mostRecentAvailable := mostRecentPricesAvailable()

	log.Info().
		Str("symbol", t.TickerSymbol).
		Str("mostRecentDailyDate", mostRecentDailyDate).
		Str("mostRecentAvailable", mostRecentAvailable).
		Msg("check if new EOD available for ticker")

	if mostRecentDailyDate < mostRecentAvailable {
		_, err = updateMarketstackTicker(awssess, db, t.TickerSymbol)
		if err != nil {
			return false, err
		}
		log.Info().
			Str("symbol", t.TickerSymbol).
			Int64("tickerId", t.TickerId).
			Msg("Updated ticker with latest EOD prices")
		return true, nil
	}

	return false, nil
}

// see if we need to pull intradays for the selected date:
//  if we don't have the intraday prices for the selected date
//  AND it was a prior business day or today and it's now 7pm or later
func (t Ticker) updateIntradays(awssess *session.Session, db *sqlx.DB, intradate string) (bool, error) {
	haveIntradayData, err := gotIntradayData(db, t.TickerId, intradate)
	if err != nil {
		return false, err
	}
	if haveIntradayData {
		return false, nil
	}

	exchange, err := getExchangeById(db, t.ExchangeId)
	if err != nil {
		return false, err
	}

	mostRecentAvailable := mostRecentPricesAvailable()

	log.Info().
		Str("symbol", t.TickerSymbol).
		Str("acronym", exchange.ExchangeAcronym).
		Str("intradate", intradate).
		Str("mostRecentAvailable", mostRecentAvailable).
		Msg("check if intraday data available for ticker")

	if intradate <= mostRecentAvailable {
		err = updateMarketstackIntraday(awssess, db, t, exchange, intradate)
		if err != nil {
			return false, err
		}
		log.Info().
			Str("symbol", t.TickerSymbol).
			Int64("tickerId", t.TickerId).
			Str("intradate", intradate).
			Msg("Updated ticker with intraday prices")
		return true, nil
	}

	return false, nil
}

// misc -----------------------------------------------------------------------

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
