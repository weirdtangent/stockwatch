package main

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/mytime"
)

type MarketIndex struct {
	MarketIndexId     int64  `db:"marketindex_id"`
	MarketIndexSymbol string `db:"marketindex_symbol"`
	MarketIndexMic    string `db:"marketindex_mic"`
	MarketIndexName   string `db:"marketindex_name"`
	CountryId         int64  `db:"country_id"`
	HasIntraday       bool   `db:"marketindex_has_intraday"`
	HasEOD            bool   `db:"marketindex_has_eod"`
	CurrencyId        int64  `db:"currency_id"`
	CreateDatetime    string `db:"create_datetime"`
	UpdateDatetime    string `db:"update_datetime"`
}

func (mi MarketIndex) LoadDailies(db *sqlx.DB, days int) ([]MarketIndexDaily, error) {
	var daily MarketIndexDaily
	dailies := make([]MarketIndexDaily, 0, days)

	rows, err := db.Queryx(
		`SELECT * FROM (
       SELECT * FROM marketindex_daily WHERE marketindex_id=? AND volume > 0
         ORDER BY price_date DESC LIMIT ?) DT1
     ORDER BY price_date`,
		mi.MarketIndexId, days)
	if err != nil {
		return dailies, err
	}
	defer rows.Close()

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
		return dailies, err
	}

	return dailies, nil
}

func (mi MarketIndex) LoadMarketIndexIntraday(db *sqlx.DB, intradate string) ([]MarketIndexIntraday, error) {
	var marketindex_intraday MarketIndexIntraday
	marketindex_intradays := make([]MarketIndexIntraday, 0)

	rows, err := db.Queryx(
		`SELECT * FROM marketindex_intraday
                 WHERE marketindex_id=? AND price_date LIKE ? AND volume > 0
                 ORDER BY price_date`,
		mi.MarketIndexId, intradate+"%")
	if err != nil {
		return marketindex_intradays, err
	}
	defer rows.Close()

	// add pre-closing price
	priorBusinessDay, err := mytime.PriorBusinessDayStr(intradate + " 21:05:00")
	if err != nil {
		return marketindex_intradays, fmt.Errorf("failed to get prior business day date")
	}
	preDaily, err := getMarketIndexDaily(db, mi.MarketIndexId, priorBusinessDay)
	if err == nil {
		marketindex_intradays = append(marketindex_intradays, MarketIndexIntraday{0, mi.MarketIndexId, priorBusinessDay, preDaily.ClosePrice, 0, "", ""})
	} else {
		log.Info().Msg("PriorBusinessDay close price was NOT included")
	}

	// add these marketindex intraday prices
	for rows.Next() {
		err = rows.StructScan(&marketindex_intraday)
		if err != nil {
			log.Warn().Err(err).
				Str("table_name", "marketindex_intraday").
				Msg("Error reading result rows")
		} else {
			marketindex_intradays = append(marketindex_intradays, marketindex_intraday)
		}
	}
	if err := rows.Err(); err != nil {
		return marketindex_intradays, err
	}

	// add post-opening price
	nextBusinessDay, err := mytime.NextBusinessDayStr(intradate + " 13:55:00")
	if err != nil {
		return marketindex_intradays, fmt.Errorf("failed to get next business day date")
	}
	postDaily, err := getMarketIndexDaily(db, mi.MarketIndexId, nextBusinessDay)
	if err == nil {
		marketindex_intradays = append(marketindex_intradays, MarketIndexIntraday{0, mi.MarketIndexId, nextBusinessDay, postDaily.OpenPrice, 0, "", ""})
	} else {
		log.Info().Msg("NextBusinessDay open price was NOT included")
	}

	return marketindex_intradays, nil
}
