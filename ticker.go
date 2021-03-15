package main

import (
	"context"
	"encoding/json"
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

type TickersEODTask struct {
	TaskAction string `json:"action"`
	TickerId   int64  `json:"ticker_id"`
	DaysBack   int    `json:"days_back"`
	Offset     int    `json:"offset"`
}

func (t *Ticker) getBySymbol(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=?", t.TickerSymbol).StructScan(t)
	return err
}

func (t *Ticker) create(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	if t.TickerSymbol == "" {
		return fmt.Errorf("Refusing to add ticker with blank symbol")
	}

	var insert = "INSERT INTO ticker SET ticker_symbol=?, exchange_id=?, ticker_name=?"
	res, err := db.Exec(insert, t.TickerSymbol, t.ExchangeId, t.TickerName)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "ticker").
			Str("ticker", t.TickerSymbol).
			Msg("Failed on INSERT")
		return err
	}
	t.TickerId, err = res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "ticker").
			Str("symbol", t.TickerSymbol).
			Msg("Failed on LAST_INSERTID")
		return err
	}
	return err
}

func (t *Ticker) getOrCreate(ctx context.Context) error {
	err := t.getBySymbol(ctx)
	if err != nil && t.TickerId == 0 {
		return t.create(ctx)
	}
	return err
}

func (t *Ticker) createOrUpdate(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	if t.TickerSymbol == "" {
		return fmt.Errorf("Refusing to add ticker with blank symbol")
	}

	err := t.getBySymbol(ctx)
	if err != nil {
		return t.create(ctx)
	}

	var update = "UPDATE ticker SET exchange_id=?, ticker_name=? WHERE ticker_id=?"
	_, err = db.Exec(update, t.ExchangeId, t.TickerName, t.TickerId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "ticker").
			Str("ticker", t.TickerSymbol).
			Msg("Failed on UPDATE")
	}
	return t.getBySymbol(ctx)
}

func (t *Ticker) getMostRecentPrice(ctx context.Context) (*TickerDaily, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var tickerDaily TickerDaily
	err := db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_id=? ORDER BY price_date DESC LIMIT 1`, t.TickerId).StructScan(&tickerDaily)
	return &tickerDaily, err
}

func (t Ticker) EarliestEOD(db *sqlx.DB) (string, float32, error) {
	type Earliest struct {
		date  string
		price float32
	}
	var earliest Earliest
	err := db.QueryRowx("SELECT price_date, close_price FROM ticker_daily WHERE ticker_id=? ORDER BY price_date LIMIT 1", t.TickerId).StructScan(&earliest)
	return earliest.date, earliest.price, err
}

func (t Ticker) ScheduleEODUpdate(awssess *session.Session, db *sqlx.DB) bool {
	earliest, _, err := t.EarliestEOD(db)

	if len(earliest) > 0 {
		if days, err := mytime.DaysAgo(earliest); err != nil || days > 900 {
			return false
		}
	}

	taskAction := "eod"

	// submit task for 1000 EODs
	taskVars := TickersEODTask{taskAction, t.TickerId, 1000, 0}
	taskJSON, err := json.Marshal(taskVars)
	log.Info().Msg(string(taskJSON))
	if err != nil {
		log.Error().Err(err).
			Int64("ticker_id", t.TickerId).
			Msg("Failed to create task JSON for EOD update for ticker")
		return false
	}

	_, err = sendNotification(awssess, "tickers", taskAction, string(taskJSON))
	if err == nil {
		log.Info().
			Int64("ticker_id", t.TickerId).
			Msg("Sent task notification for EOD update for ticker")
	}

	// submit task for 1000 more EODs
	taskVars = TickersEODTask{taskAction, t.TickerId, 1000, 1000}
	taskJSON, err = json.Marshal(taskVars)
	log.Info().Msg(string(taskJSON))
	if err != nil {
		log.Error().Err(err).
			Int64("ticker_id", t.TickerId).
			Msg("Failed to create task JSON for EOD update for ticker")
		return true
	}

	_, err = sendNotification(awssess, "tickers", taskAction, string(taskJSON))
	if err == nil {
		log.Info().
			Int64("ticker_id", t.TickerId).
			Msg("Sent task notification for EOD update for ticker")
	}
	return true
}

func (t Ticker) LoadTickerDailies(db *sqlx.DB, days int) ([]TickerDaily, error) {
	var ticker_daily TickerDaily
	fromDate := mytime.DateStr(days * -1)
	dailies := make([]TickerDaily, 0, days)

	rows, err := db.Queryx(
		`SELECT * FROM (
       SELECT * FROM ticker_daily WHERE ticker_id=? AND volume > 0 AND price_date > ?
		   ORDER BY price_date DESC) DT1
		 ORDER BY price_date`,
		t.TickerId, fromDate)
	if err != nil {
		return dailies, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&ticker_daily)
		if err != nil {
			log.Warn().Err(err).
				Str("table_name", "ticker_daily").
				Msg("Error reading result rows")
		} else {
			dailies = append(dailies, ticker_daily)
		}
	}
	if err := rows.Err(); err != nil {
		return dailies, err
	}

	return dailies, nil
}

func (t Ticker) LoadTickerIntraday(db *sqlx.DB, intradate string) ([]TickerIntraday, error) {
	var intraday TickerIntraday
	intradays := make([]TickerIntraday, 0)

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
	preTickerDaily, err := getTickerDaily(db, t.TickerId, priorBusinessDay)
	if err == nil {
		intradays = append(intradays, TickerIntraday{0, t.TickerId, priorBusinessDay, preTickerDaily.ClosePrice, 0, "", ""})
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
	postTickerDaily, err := getTickerDaily(db, t.TickerId, nextBusinessDay)
	if err == nil {
		intradays = append(intradays, TickerIntraday{0, t.TickerId, nextBusinessDay, postTickerDaily.OpenPrice, 0, "", ""})
	} else {
		log.Info().Msg("NextBusinessDay open price was NOT included")
	}

	return intradays, nil
}

// see if we need to pull a ticker daily update:
//  if we don't have the EOD price for the prior business day
//  OR if we don't have it for the current business day and it's now 7pm or later
func (t *Ticker) updateTickerPrices(ctx context.Context) (bool, error) {
	logger := log.Ctx(ctx)

	mostRecentPrice, err := t.getMostRecentPrice(ctx)
	if err != nil {
		mostRecentPrice = &TickerDaily{}
	}
	mostRecentPriceDate := mostRecentPrice.PriceDate
	mostRecentAvailable := mostRecentPricesAvailable()

	logger.Info().
		Str("ticker", t.TickerSymbol).
		Str("most_recent_price_date", mostRecentPriceDate).
		Str("most_recent_available", mostRecentAvailable).
		Msg("check if new EOD available for ticker")

	if mostRecentPriceDate < mostRecentAvailable {
		err = loadTickerPrices(ctx, t)
		if err != nil {
			return false, err
		}
		logger.Info().
			Str("ticker", t.TickerSymbol).
			Msg("Updated ticker with latest EOD prices")
		return true, nil
	}

	return false, nil
}

// see if we need to pull intradays for the selected date:
//  if we don't have the intraday prices for the selected date
//  AND it was a prior business day or today and it's now 7pm or later
func (t Ticker) updateTickerIntradays(ctx context.Context, intradate string) (bool, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)

	haveTickerIntradayData, err := gotTickerIntradayData(db, t.TickerId, intradate)
	if err != nil {
		return false, err
	}
	if haveTickerIntradayData {
		return false, nil
	}

	exchange, err := getExchangeById(ctx, t.ExchangeId)
	if err != nil {
		return false, err
	}

	mostRecentAvailable := mostRecentPricesAvailable()

	logger.Info().
		Str("symbol", t.TickerSymbol).
		Str("acronym", exchange.ExchangeAcronym).
		Str("intradate", intradate).
		Str("mostRecentAvailable", mostRecentAvailable).
		Msg("check if intraday data available for ticker")

	if intradate <= mostRecentAvailable {
		//err = fetchTickerIntraday(ctx, t, exchange, intradate)
		err := fmt.Errorf("")
		if err != nil {
			return false, err
		}
		logger.Info().
			Str("symbol", t.TickerSymbol).
			Int64("tickerId", t.TickerId).
			Str("intradate", intradate).
			Msg("Updated ticker with intraday prices")
		return true, nil
	}

	return false, nil
}

// misc -----------------------------------------------------------------------

func getTickerBySymbol(ctx context.Context, symbol string) (*Ticker, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var ticker Ticker
	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=?", symbol).StructScan(&ticker)
	return &ticker, err
}
