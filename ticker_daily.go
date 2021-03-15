package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type TickerDaily struct {
	TickerDailyId  int64   `db:"ticker_daily_id"`
	TickerId       int64   `db:"ticker_id"`
	PriceDate      string  `db:"price_date"`
	OpenPrice      float32 `db:"open_price"`
	HighPrice      float32 `db:"high_price"`
	LowPrice       float32 `db:"low_price"`
	ClosePrice     float32 `db:"close_price"`
	Volume         float32 `db:"volume"`
	CreateDatetime string  `db:"create_datetime"`
	UpdateDatetime string  `db:"update_datetime"`
}

type TickerDailies struct {
	Days []TickerDaily
}

type ByTickerPriceDate TickerDailies

func (a ByTickerPriceDate) Len() int { return len(a.Days) }
func (a ByTickerPriceDate) Less(i, j int) bool {
	return a.Days[i].PriceDate < a.Days[j].PriceDate
}
func (a ByTickerPriceDate) Swap(i, j int) { a.Days[i], a.Days[j] = a.Days[j], a.Days[i] }

func (td TickerDailies) Sort() *TickerDailies {
	sort.Sort(ByTickerPriceDate(td))
	return &td
}

func (td TickerDailies) Reverse() *TickerDailies {
	sort.Sort(sort.Reverse(ByTickerPriceDate(td)))
	return &td
}

func (td TickerDailies) Count() int {
	return len(td.Days)
}

func (td *TickerDaily) getByDate(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_id=? AND price_date=?`, td.TickerId, td.PriceDate).StructScan(td)
	return err
}

func (td *TickerDaily) createIfNew(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)
	logger := log.Ctx(ctx)

	if td.Volume == 0 {
		return fmt.Errorf("Refusing to add ticker daily with 0 volume")
	}

	err := td.getByDate(ctx)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_daily SET ticker_id=?, price_date=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=?"
	_, err = db.Exec(insert, td.TickerId, td.PriceDate, td.OpenPrice, td.HighPrice, td.LowPrice, td.ClosePrice, td.Volume)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker_daily").
			Msg("Failed on INSERT")
	}
	return err
}

func getTickerDaily(db *sqlx.DB, ticker_id int64, daily_date string) (*TickerDaily, error) {
	var tickerdaily TickerDaily
	if len(daily_date) > 10 {
		daily_date = daily_date[0:10]
	}
	err := db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_id=? AND price_date=?`, ticker_id, daily_date).StructScan(&tickerdaily)
	return &tickerdaily, err
}

func getTickerDailyById(db *sqlx.DB, ticker_daily_id int64) (*TickerDaily, error) {
	var tickerdaily TickerDaily
	err := db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_daily_id=?`, ticker_daily_id).StructScan(&tickerdaily)
	return &tickerdaily, err
}

func getTickerDailyMostRecent(db *sqlx.DB, ticker_id int64) (*TickerDaily, error) {
	var tickerdaily TickerDaily
	err := db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_id=? ORDER BY price_date DESC LIMIT 1`, ticker_id).StructScan(&tickerdaily)
	return &tickerdaily, err
}

func getLastTickerDailyMove(db *sqlx.DB, ticker_id int64) (string, error) {
	var lastTickerDailyMove string
	row := db.QueryRowx(
		`SELECT IF(ticker_daily.close_price >= prev.close_price,"up",IF(ticker_daily.close_price < prev.close_price,"down","unknown")) AS lastTickerDailyMove
		 FROM ticker_daily
		 LEFT JOIN (
		   SELECT ticker_id, close_price FROM ticker_daily AS prev_ticker_daily
			 WHERE ticker_id=? ORDER by price_date DESC LIMIT 1,1
		 ) AS prev ON (ticker_daily.ticker_id = prev.ticker_id)
		 WHERE ticker_daily.ticker_id=? ORDER BY price_date DESC LIMIT 1`,
		ticker_id, ticker_id)
	err := row.Scan(&lastTickerDailyMove)
	return lastTickerDailyMove, err
}
