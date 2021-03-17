package main

import (
	"fmt"
	"sort"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type TickerIntraday struct {
	TickerIntradayId int64   `db:"intraday_id"`
	TickerId         int64   `db:"ticker_id"`
	PriceDate        string  `db:"price_date"`
	LastPrice        float64 `db:"last_price"`
	Volume           float64 `db:"volume"`
	CreateDatetime   string  `db:"create_datetime"`
	UpdateDatetime   string  `db:"update_datetime"`
}

type TickerIntradays struct {
	Moments []TickerIntraday
}

type ByTickerPriceTime TickerIntradays

func (a ByTickerPriceTime) Len() int { return len(a.Moments) }
func (a ByTickerPriceTime) Less(i, j int) bool {
	return a.Moments[i].PriceDate < a.Moments[j].PriceDate
}
func (a ByTickerPriceTime) Swap(i, j int) { a.Moments[i], a.Moments[j] = a.Moments[j], a.Moments[i] }

func (i TickerIntradays) Sort() *TickerIntradays {
	sort.Sort(ByTickerPriceTime(i))
	return &i
}

func (i TickerIntradays) Reverse() *TickerIntradays {
	sort.Sort(sort.Reverse(ByTickerPriceTime(i)))
	return &i
}

func (i TickerIntradays) Count() int {
	return len(i.Moments)
}

func getTickerIntraday(db *sqlx.DB, ticker_id int64, ticker_intraday_date string) (*TickerIntraday, error) {
	var ticker_intraday TickerIntraday
	err := db.QueryRowx(
		`SELECT * FROM ticker_intraday WHERE ticker_id=? AND price_date=?`,
		ticker_id, ticker_intraday_date).StructScan(&ticker_intraday)
	return &ticker_intraday, err
}

func getTickerIntradayById(db *sqlx.DB, ticker_intraday_id int64) (*TickerIntraday, error) {
	var ticker_intraday TickerIntraday
	err := db.QueryRowx(
		`SELECT * FROM ticker_intraday WHERE ticker_intraday_id=?`,
		ticker_intraday_id).StructScan(&ticker_intraday)
	return &ticker_intraday, err
}

func gotTickerIntradayData(db *sqlx.DB, ticker_id int64, intradate string) (bool, error) {
	var count int
	err := db.QueryRowx(
		`SELECT COUNT(*) FROM ticker_intraday WHERE ticker_id=?
		 AND price_date LIKE ?
		 ORDER BY price_date LIMIT 1`,
		ticker_id, intradate+"%").Scan(&count)
	log.Warn().Msg(fmt.Sprintf("Checking, I have %d ticker_intraday moments for %s", count, intradate))

	// if we have at least 50, we won't automatically update this intradate anymore
	return count >= 50, err
}

func createTickerIntraday(db *sqlx.DB, ticker_intraday *TickerIntraday) (*TickerIntraday, error) {
	var insert = "INSERT INTO ticker_intraday SET ticker_id=?, price_date=?, last_price=?, volume=?"

	res, err := db.Exec(insert, ticker_intraday.TickerId, ticker_intraday.PriceDate, ticker_intraday.LastPrice, ticker_intraday.Volume)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "ticker_intraday").
			Msg("Failed on INSERT")
	}
	ticker_intraday_id, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "ticker_intraday").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getTickerIntradayById(db, ticker_intraday_id)
}

func getOrCreateTickerIntraday(db *sqlx.DB, ticker_intraday *TickerIntraday) (*TickerIntraday, error) {
	existing, err := getTickerIntraday(db, ticker_intraday.TickerId, ticker_intraday.PriceDate)
	if err != nil && existing.TickerIntradayId == 0 {
		return createTickerIntraday(db, ticker_intraday)
	}
	return existing, err
}

func createOrUpdateTickerIntraday(db *sqlx.DB, ticker_intraday *TickerIntraday) (*TickerIntraday, error) {
	var update = "UPDATE ticker_intraday SET last_price=?, volume=? WHERE ticker_id=? AND price_date=?"

	existing, err := getTickerIntraday(db, ticker_intraday.TickerId, ticker_intraday.PriceDate)
	if err != nil {
		return createTickerIntraday(db, ticker_intraday)
	}

	_, err = db.Exec(update, ticker_intraday.LastPrice, ticker_intraday.Volume, existing.TickerId, existing.PriceDate)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "ticker_intraday").
			Msg("Failed on UPDATE")
	}
	return getTickerIntradayById(db, existing.TickerIntradayId)
}
