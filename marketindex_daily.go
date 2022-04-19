package main

import (
	"sort"

	"github.com/jmoiron/sqlx"
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
