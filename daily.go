package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func getDaily(db *sqlx.DB, ticker_id int64, daily_date string) (*Daily, error) {
	var daily Daily
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

func loadDailies(db *sqlx.DB, ticker_id int64, days int) ([]Daily, error) {
	rows, err := db.Queryx(
		`SELECT * FROM (
       SELECT * FROM daily WHERE ticker_id=? AND volume > 0
		   ORDER BY price_date DESC LIMIT ?) DT1
		 ORDER BY price_date`,
		ticker_id, days)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "daily").
			Msg("Failed on SELECT")
	}
	defer rows.Close()

	var daily Daily
	dailies := make([]Daily, 0, days)
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
		log.Fatal().Err(err).
			Str("table_name", "daily").
			Msg("Error reading result rows")
	}

	return dailies, err
}

func createDaily(db *sqlx.DB, daily *Daily) (*Daily, error) {
	var insert = "INSERT INTO daily SET ticker_id=?, price_date=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=?"

	res, err := db.Exec(insert, daily.Ticker_id, daily.Price_date, daily.Open_price, daily.High_price, daily.Low_price, daily.Close_price, daily.Volume)
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
	existing, err := getDaily(db, daily.Ticker_id, daily.Price_date)
	if err != nil && existing.Daily_id == 0 {
		return createDaily(db, daily)
	}
	return existing, err
}

func createOrUpdateDaily(db *sqlx.DB, daily *Daily) (*Daily, error) {
	var update = "UPDATE daily SET open_price=?, high_price=?, low_price=?, close_price=?, volume=? WHERE ticker_id=? AND price_date=?"

	existing, err := getDaily(db, daily.Ticker_id, daily.Price_date)
	if err != nil {
		return createDaily(db, daily)
	}

	_, err = db.Exec(update, daily.Open_price, daily.High_price, daily.Low_price, daily.Close_price, daily.Volume, existing.Ticker_id, existing.Price_date)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "daily").
			Msg("Failed on UPDATE")
	}
	return getDailyById(db, existing.Daily_id)
}
