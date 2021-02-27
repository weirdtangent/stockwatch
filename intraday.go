package main

import (
	"fmt"
	"sort"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Intradays struct {
	Moments []Intraday
}

type ByPriceTime Intradays

func (a ByPriceTime) Len() int           { return len(a.Moments) }
func (a ByPriceTime) Less(i, j int) bool { return a.Moments[i].PriceDate < a.Moments[j].PriceDate }
func (a ByPriceTime) Swap(i, j int)      { a.Moments[i], a.Moments[j] = a.Moments[j], a.Moments[i] }

func (i Intradays) Sort() *Intradays {
	sort.Sort(ByPriceTime(i))
	return &i
}

func (i Intradays) Reverse() *Intradays {
	sort.Sort(sort.Reverse(ByPriceTime(i)))
	return &i
}

func (i Intradays) Count() int {
	return len(i.Moments)
}

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
	log.Warn().Msg(fmt.Sprintf("Checking, I have %d intraday moments for %s", count, intradate))

	// if we have at least 50, we won't automatically update this intradate anymore
	return count >= 50, err
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
