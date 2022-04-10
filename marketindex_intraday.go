package main

import (
	"sort"
)

type MarketIndexIntraday struct {
	MarketIndexIntradayId int64   `db:"intraday_id"`
	TickerId              int64   `db:"ticker_id"`
	PriceDate             string  `db:"price_date"`
	LastPrice             float64 `db:"last_price"`
	Volume                float64 `db:"volume"`
	CreateDatetime        string  `db:"create_datetime"`
	UpdateDatetime        string  `db:"update_datetime"`
}

type MarketIndexIntradays struct {
	Moments []MarketIndexIntraday
}

type ByMarketIndexPriceTime MarketIndexIntradays

func (a ByMarketIndexPriceTime) Len() int { return len(a.Moments) }
func (a ByMarketIndexPriceTime) Less(i, j int) bool {
	return a.Moments[i].PriceDate < a.Moments[j].PriceDate
}
func (a ByMarketIndexPriceTime) Swap(i, j int) {
	a.Moments[i], a.Moments[j] = a.Moments[j], a.Moments[i]
}

func (i MarketIndexIntradays) Sort() *MarketIndexIntradays {
	sort.Sort(ByMarketIndexPriceTime(i))
	return &i
}

func (i MarketIndexIntradays) Reverse() *MarketIndexIntradays {
	sort.Sort(sort.Reverse(ByMarketIndexPriceTime(i)))
	return &i
}

func (i MarketIndexIntradays) Count() int {
	return len(i.Moments)
}

// func getMarketIndexIntraday(db *sqlx.DB, marketindex_id int64, marketindex_intraday_date string) (*MarketIndexIntraday, error) {
// 	var marketindex_intraday MarketIndexIntraday
// 	err := db.QueryRowx(
// 		`SELECT * FROM marketindex_intraday WHERE marketindex_id=? AND price_date=?`,
// 		marketindex_id, marketindex_intraday_date).StructScan(&marketindex_intraday)
// 	return &marketindex_intraday, err
// }

// func getMarketIndexIntradayById(db *sqlx.DB, marketindex_intraday_id int64) (*MarketIndexIntraday, error) {
// 	var marketindex_intraday MarketIndexIntraday
// 	err := db.QueryRowx(
// 		`SELECT * FROM marketindex_intraday WHERE marketindex_intraday_id=?`,
// 		marketindex_intraday_id).StructScan(&marketindex_intraday)
// 	return &marketindex_intraday, err
// }

// func gotMarketIndexIntradayData(db *sqlx.DB, marketindex_id int64, intradate string) (bool, error) {
// 	var count int
// 	err := db.QueryRowx(
// 		`SELECT COUNT(*) FROM marketindex_intraday WHERE marketindex_id=?
// 		 AND price_date LIKE ?
// 		 ORDER BY price_date LIMIT 1`,
// 		marketindex_id, intradate+"%").Scan(&count)
// 	log.Warn().Msg(fmt.Sprintf("Checking, I have %d marketindex_intraday moments for %s", count, intradate))

// 	// if we have at least 50, we won't automatically update this intradate anymore
// 	return count >= 50, err
// }

// func createMarketIndexIntraday(db *sqlx.DB, marketindex_intraday *MarketIndexIntraday) (*MarketIndexIntraday, error) {
// 	var insert = "INSERT INTO marketindex_intraday SET marketindex_id=?, price_date=?, last_price=?, volume=?"

// 	res, err := db.Exec(insert, marketindex_intraday.MarketIndexIntradayId, marketindex_intraday.PriceDate, marketindex_intraday.LastPrice, marketindex_intraday.Volume)
// 	if err != nil {
// 		log.Fatal().Err(err).
// 			Str("table_name", "marketindex_intraday").
// 			Msg("Failed on INSERT")
// 	}
// 	marketindex_intraday_id, err := res.LastInsertId()
// 	if err != nil {
// 		log.Fatal().Err(err).
// 			Str("table_name", "marketindex_intraday").
// 			Msg("Failed on LAST_INSERT_ID")
// 	}
// 	return getMarketIndexIntradayById(db, marketindex_intraday_id)
// }

// func getOrCreateMarketIndexIntraday(db *sqlx.DB, marketindex_intraday *MarketIndexIntraday) (*MarketIndexIntraday, error) {
// 	existing, err := getMarketIndexIntraday(db, marketindex_intraday.MarketIndexIntradayId, marketindex_intraday.PriceDate)
// 	if err != nil && existing.MarketIndexIntradayId == 0 {
// 		return createMarketIndexIntraday(db, marketindex_intraday)
// 	}
// 	return existing, err
// }

// func createOrUpdateMarketIndexIntraday(db *sqlx.DB, marketindex_intraday *MarketIndexIntraday) (*MarketIndexIntraday, error) {
// 	var update = "UPDATE marketindex_intraday SET last_price=?, volume=? WHERE marketindex_id=? AND price_date=?"

// 	existing, err := getMarketIndexIntraday(db, marketindex_intraday.MarketIndexIntradayId, marketindex_intraday.PriceDate)
// 	if err != nil {
// 		return createMarketIndexIntraday(db, marketindex_intraday)
// 	}

// 	_, err = db.Exec(update, marketindex_intraday.LastPrice, marketindex_intraday.Volume, existing.MarketIndexIntradayId, existing.PriceDate)
// 	if err != nil {
// 		log.Warn().Err(err).
// 			Str("table_name", "marketindex_intraday").
// 			Msg("Failed on UPDATE")
// 	}
// 	return getMarketIndexIntradayById(db, existing.MarketIndexIntradayId)
// }
