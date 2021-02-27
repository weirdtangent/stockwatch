package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/mytime"
)

func getIntraday(db *sqlx.DB, ticker_id int64, intraday_date string) (*Intraday, error) {
	var intraday Intraday
	err := db.QueryRowx(
		`SELECT * FROM intraday WHERE ticker_id=? AND price_date=?`,
		ticker_id, intraday_date).StructScan(&intraday)
	return &intraday, err
}

func getIntradayById(db *sqlx.DB, intraday_id int64) (*Intraday, error) {
	var intraday Intraday
	err := db.QueryRowx(
		`SELECT * FROM intraday WHERE intraday_id=?`,
		intraday_id).StructScan(&intraday)
	return &intraday, err
}

func gotIntradayData(db *sqlx.DB, ticker_id int64, intradate string) (bool, error) {
	var count int
	err := db.QueryRowx(
		`SELECT COUNT(*) FROM intraday WHERE ticker_id=?
		 AND price_date LIKE ?
		 ORDER BY price_date LIMIT 1`,
		ticker_id, intradate+"%").Scan(&count)
	return count >= 74, err
}

func loadIntradayData(db *sqlx.DB, ticker_id int64, intradate string) ([]Intraday, error) {
	rows, err := db.Queryx(
		`SELECT * FROM intraday
		 WHERE ticker_id=? AND price_date LIKE ? AND volume > 0
		 ORDER BY price_date`,
		ticker_id, intradate+"%")
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "intraday").
			Msg("Failed on SELECT")
	}
	defer rows.Close()

	var intraday Intraday
	// 9:30am - 4:00pm, every 5 min = 78 intraday prices
	// plus we will add the closing date from the day before
	// and the opening price on the next day (if we have them)
	intradays := make([]Intraday, 0, 80)

	// add pre-closing price
	priorBusinessDay, err := mytime.PriorBusinessDayStr(intradate + " 21:05:00")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get prior business day")
	}
	preDaily, err := getDaily(db, ticker_id, priorBusinessDay)
	if err == nil {
		intradays = append(intradays, Intraday{0, ticker_id, priorBusinessDay, preDaily.ClosePrice, 0, "", ""})
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
		log.Fatal().Err(err).
			Str("table_name", "intraday").
			Msg("Error reading result rows")
	}

	// add post-opening price
	nextBusinessDay, err := mytime.NextBusinessDayStr(intradate + " 13:55:00")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get next business day")
	}
	postDaily, err := getDaily(db, ticker_id, nextBusinessDay)
	if err == nil {
		intradays = append(intradays, Intraday{0, ticker_id, nextBusinessDay, postDaily.OpenPrice, 0, "", ""})
	} else {
		log.Info().Msg("NextBusinessDay open price was NOT included")
	}

	return intradays, nil
}

func createIntraday(db *sqlx.DB, intraday *Intraday) (*Intraday, error) {
	var insert = "INSERT INTO intraday SET ticker_id=?, price_date=?, last_price=?, volume=?"

	res, err := db.Exec(insert, intraday.TickerId, intraday.PriceDate, intraday.LastPrice, intraday.Volume)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "intraday").
			Msg("Failed on INSERT")
	}
	intraday_id, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "intraday").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getIntradayById(db, intraday_id)
}

func getOrCreateIntraday(db *sqlx.DB, intraday *Intraday) (*Intraday, error) {
	existing, err := getIntraday(db, intraday.TickerId, intraday.PriceDate)
	if err != nil && existing.IntradayId == 0 {
		return createIntraday(db, intraday)
	}
	return existing, err
}

func createOrUpdateIntraday(db *sqlx.DB, intraday *Intraday) (*Intraday, error) {
	var update = "UPDATE intraday SET last_price=?, volume=? WHERE ticker_id=? AND price_date=?"

	existing, err := getIntraday(db, intraday.TickerId, intraday.PriceDate)
	if err != nil {
		return createIntraday(db, intraday)
	}

	_, err = db.Exec(update, intraday.LastPrice, intraday.Volume, existing.TickerId, existing.PriceDate)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "intraday").
			Msg("Failed on UPDATE")
	}
	return getIntradayById(db, existing.IntradayId)
}
