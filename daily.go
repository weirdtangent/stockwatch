package main

import (
	"github.com/rs/zerolog/log"
)

func getDaily(ticker_id int64, daily_date string) (*Daily, error) {
	var daily Daily
	err := db_session.QueryRowx("SELECT * FROM daily WHERE ticker_id=? AND price_date=?", ticker_id, daily_date).StructScan(&daily)
	return &daily, err
}

func getDailyById(daily_id int64) (*Daily, error) {
	var daily Daily
	err := db_session.QueryRowx("SELECT * FROM daily WHERE daily_id=?", daily_id).StructScan(&daily)
	return &daily, err
}

func getDailyMostRecent(ticker_id int64) (*Daily, error) {
	var daily Daily
	err := db_session.QueryRowx("SELECT * FROM daily WHERE ticker_id=? ORDER BY price_date DESC LIMIT 1", ticker_id).StructScan(&daily)
	return &daily, err
}

func loadDailies(ticker_id int64, days int) ([]Daily, error) {
	rows, err := db_session.Queryx("SELECT * FROM daily WHERE ticker_id=? AND volume > 0 ORDER BY price_date DESC LIMIT ?", ticker_id, days)
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

func createDaily(daily *Daily) (*Daily, error) {
	var insert = "INSERT INTO daily SET ticker_id=?, price_date=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=?"

	res, err := db_session.Exec(insert, daily.Ticker_id, daily.Price_date, daily.Open_price, daily.High_price, daily.Low_price, daily.Close_price, daily.Volume)
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
	return getDailyById(daily_id)
}

func getOrCreateDaily(daily *Daily) (*Daily, error) {
	existing, err := getDaily(daily.Ticker_id, daily.Price_date)
	if err != nil && existing.Daily_id == 0 {
		return createDaily(daily)
	}
	return existing, err
}

func createOrUpdateDaily(daily *Daily) (*Daily, error) {
	var update = "UPDATE daily SET open_price=?, high_price=?, low_price=?, close_price=?, volume=? WHERE ticker_id=? AND price_date=?"

	existing, err := getDaily(daily.Ticker_id, daily.Price_date)
	if err != nil {
		return createDaily(daily)
	}

	_, err = db_session.Exec(update, daily.Open_price, daily.High_price, daily.Low_price, daily.Close_price, daily.Volume, existing.Ticker_id, existing.Price_date)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "daily").
			Msg("Failed on UPDATE")
	}
	return getDailyById(existing.Daily_id)
}
