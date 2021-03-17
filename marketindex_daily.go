package main

import (
	"sort"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type MarketIndexDaily struct {
	MarketIndexDailyId int64   `db:"marketindex_daily_id"`
	MarketIndexId      int64   `db:"marketindex_id"`
	PriceDate          string  `db:"price_date"`
	OpenPrice          float64 `db:"open_price"`
	HighPrice          float64 `db:"high_price"`
	LowPrice           float64 `db:"low_price"`
	ClosePrice         float64 `db:"close_price"`
	Volume             float64 `db:"volume"`
	CreateDatetime     string  `db:"create_datetime"`
	UpdateDatetime     string  `db:"update_datetime"`
}

type MarketIndexDailies struct {
	Days []MarketIndexDaily
}

type MarketIndexByPriceDate MarketIndexDailies

func (a MarketIndexByPriceDate) Len() int           { return len(a.Days) }
func (a MarketIndexByPriceDate) Less(i, j int) bool { return a.Days[i].PriceDate < a.Days[j].PriceDate }
func (a MarketIndexByPriceDate) Swap(i, j int)      { a.Days[i], a.Days[j] = a.Days[j], a.Days[i] }

func (mi MarketIndexDailies) Sort() *MarketIndexDailies {
	sort.Sort(MarketIndexByPriceDate(mi))
	return &mi
}

func (mi MarketIndexDailies) Reverse() *MarketIndexDailies {
	sort.Sort(sort.Reverse(MarketIndexByPriceDate(mi)))
	return &mi
}

func (mi MarketIndexDailies) Count() int {
	return len(mi.Days)
}

func getMarketIndexDaily(db *sqlx.DB, marketindex_id int64, daily_date string) (*MarketIndexDaily, error) {
	var marketindexdaily MarketIndexDaily
	if len(daily_date) > 10 {
		daily_date = daily_date[0:10]
	}
	err := db.QueryRowx(
		`SELECT * FROM marketindex_daily WHERE marketindex_id=? AND price_date=?`,
		marketindex_id, daily_date).StructScan(&marketindexdaily)
	return &marketindexdaily, err
}

func getMarketIndexDailyById(db *sqlx.DB, marketindex_daily_id int64) (*MarketIndexDaily, error) {
	var marketindexdaily MarketIndexDaily
	err := db.QueryRowx(
		`SELECT * FROM marketindex_daily WHERE marketindex_daily_id=?`,
		marketindex_daily_id).StructScan(&marketindexdaily)
	return &marketindexdaily, err
}

func getMarketIndexDailyMostRecent(db *sqlx.DB, marketindex_id int64) (*MarketIndexDaily, error) {
	var marketindexdaily MarketIndexDaily
	err := db.QueryRowx(
		`SELECT * FROM marketindex_daily WHERE marketindex_id=?
		 ORDER BY price_date DESC LIMIT 1`,
		marketindex_id).StructScan(&marketindexdaily)
	return &marketindexdaily, err
}

func getLastMarketIndexDailyMove(db *sqlx.DB, marketindex_id int64) (string, error) {
	var lastMarketIndexDailyMove string
	row := db.QueryRowx(
		`SELECT IF(marketindex_daily.close_price >= prev.close_price,"up",IF(marketindex_daily.close_price < prev.close_price,"down","unknown")) AS lastMarketIndexDailyMove
		 FROM marketindex_daily
		 LEFT JOIN (
		   SELECT marketindex_id, close_price FROM marketindex_daily AS prev_marketindex_daily
			 WHERE marketindex_id=? ORDER by price_date DESC LIMIT 1,1
		 ) AS prev ON (marketindex_daily.marketindex_id = prev.marketindex_id)
		 WHERE marketindex_daily.marketindex_id=? ORDER BY price_date DESC LIMIT 1`,
		marketindex_id, marketindex_id)
	err := row.Scan(&lastMarketIndexDailyMove)
	return lastMarketIndexDailyMove, err
}

func createMarketIndexDaily(db *sqlx.DB, marketindexdaily *MarketIndexDaily) (*MarketIndexDaily, error) {
	var insert = "INSERT INTO marketindex_daily SET marketindex_id=?, price_date=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=?"

	res, err := db.Exec(insert, marketindexdaily.MarketIndexId, marketindexdaily.PriceDate, marketindexdaily.OpenPrice, marketindexdaily.HighPrice, marketindexdaily.LowPrice, marketindexdaily.ClosePrice, marketindexdaily.Volume)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "marketindex_daily").
			Msg("Failed on INSERT")
	}
	marketindex_daily_id, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "marketindex_daily").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getMarketIndexDailyById(db, marketindex_daily_id)
}

func getOrCreateMarketIndexDaily(db *sqlx.DB, marketindexdaily *MarketIndexDaily) (*MarketIndexDaily, error) {
	existing, err := getMarketIndexDaily(db, marketindexdaily.MarketIndexId, marketindexdaily.PriceDate)
	if err != nil && existing.MarketIndexDailyId == 0 {
		return createMarketIndexDaily(db, marketindexdaily)
	}
	return existing, err
}

func createOrUpdateMarketIndexDaily(db *sqlx.DB, marketindexdaily *MarketIndexDaily) (*MarketIndexDaily, error) {
	var update = "UPDATE marketindex_daily SET open_price=?, high_price=?, low_price=?, close_price=?, volume=? WHERE marketindex_id=? AND price_date=?"

	existing, err := getMarketIndexDaily(db, marketindexdaily.MarketIndexId, marketindexdaily.PriceDate)
	if err != nil {
		return createMarketIndexDaily(db, marketindexdaily)
	}

	_, err = db.Exec(update, marketindexdaily.OpenPrice, marketindexdaily.HighPrice, marketindexdaily.LowPrice, marketindexdaily.ClosePrice, marketindexdaily.Volume, existing.MarketIndexId, existing.PriceDate)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "marketindex_daily").
			Msg("Failed on UPDATE")
	}
	return getMarketIndexDailyById(db, existing.MarketIndexDailyId)
}
