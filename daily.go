package main

import (
	"sort"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Dailies struct {
	Days []Daily
}

type ByPriceDate Dailies

func (a ByPriceDate) Len() int           { return len(a.Days) }
func (a ByPriceDate) Less(i, j int) bool { return a.Days[i].PriceDate < a.Days[j].PriceDate }
func (a ByPriceDate) Swap(i, j int)      { a.Days[i], a.Days[j] = a.Days[j], a.Days[i] }

func (d Dailies) Sort() *Dailies {
	sort.Sort(ByPriceDate(d))
	return &d
}

func (d Dailies) Reverse() *Dailies {
	sort.Sort(sort.Reverse(ByPriceDate(d)))
	return &d
}

func (d Dailies) Count() int {
	return len(d.Days)
}

func getDaily(db *sqlx.DB, ticker_id int64, daily_date string) (*Daily, error) {
	var daily Daily
	if len(daily_date) > 10 {
		daily_date = daily_date[0:10]
	}
	err := db.QueryRowx(
		`SELECT * FROM daily WHERE ticker_id=? AND price_date=?`,
		ticker_id, daily_date).StructScan(&daily)
	return &daily, err
}

func getDailyById(db *sqlx.DB, daily_id int64) (*Daily, error) {
	var daily Daily
	err := db.QueryRowx(
		`SELECT * FROM daily WHERE daily_id=?`,
		daily_id).StructScan(&daily)
	return &daily, err
}

func getDailyMostRecent(db *sqlx.DB, ticker_id int64) (*Daily, error) {
	var daily Daily
	err := db.QueryRowx(
		`SELECT * FROM daily WHERE ticker_id=?
		 ORDER BY price_date DESC LIMIT 1`,
		ticker_id).StructScan(&daily)
	return &daily, err
}

func getLastDailyMove(db *sqlx.DB, ticker_id int64) (string, error) {
	var lastDailyMove string
	row := db.QueryRowx(
		`SELECT IF(daily.close_price >= prev.close_price,"up",IF(daily.close_price < prev.close_price,"down","unknown")) AS lastDailyMove
		 FROM daily
		 LEFT JOIN (
		   SELECT ticker_id, close_price FROM daily AS prev_daily
			 WHERE ticker_id=? ORDER by price_date DESC LIMIT 1,1
		 ) AS prev ON (daily.ticker_id = prev.ticker_id)
		 WHERE daily.ticker_id=? ORDER BY price_date DESC LIMIT 1`,
		ticker_id, ticker_id)
	err := row.Scan(&lastDailyMove)
	return lastDailyMove, err
}

func createDaily(db *sqlx.DB, daily *Daily) (*Daily, error) {
	var insert = "INSERT INTO daily SET ticker_id=?, price_date=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=?"

	res, err := db.Exec(insert, daily.TickerId, daily.PriceDate, daily.OpenPrice, daily.HighPrice, daily.LowPrice, daily.ClosePrice, daily.Volume)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "daily").
			Msg("Failed on INSERT")
	}
	daily_id, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "daily").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getDailyById(db, daily_id)
}

func getOrCreateDaily(db *sqlx.DB, daily *Daily) (*Daily, error) {
	existing, err := getDaily(db, daily.TickerId, daily.PriceDate)
	if err != nil && existing.DailyId == 0 {
		return createDaily(db, daily)
	}
	return existing, err
}

func createOrUpdateDaily(db *sqlx.DB, daily *Daily) (*Daily, error) {
	var update = "UPDATE daily SET open_price=?, high_price=?, low_price=?, close_price=?, volume=? WHERE ticker_id=? AND price_date=?"

	existing, err := getDaily(db, daily.TickerId, daily.PriceDate)
	if err != nil {
		return createDaily(db, daily)
	}

	_, err = db.Exec(update, daily.OpenPrice, daily.HighPrice, daily.LowPrice, daily.ClosePrice, daily.Volume, existing.TickerId, existing.PriceDate)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "daily").
			Msg("Failed on UPDATE")
	}
	return getDailyById(db, existing.DailyId)
}
