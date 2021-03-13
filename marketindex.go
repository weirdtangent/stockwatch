package main

import (
	"context"
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
		return marketindex_intradays, fmt.Errorf("Failed to get prior business day date")
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
		return marketindex_intradays, fmt.Errorf("Failed to get next business day date")
	}
	postDaily, err := getMarketIndexDaily(db, mi.MarketIndexId, nextBusinessDay)
	if err == nil {
		marketindex_intradays = append(marketindex_intradays, MarketIndexIntraday{0, mi.MarketIndexId, nextBusinessDay, postDaily.OpenPrice, 0, "", ""})
	} else {
		log.Info().Msg("NextBusinessDay open price was NOT included")
	}

	return marketindex_intradays, nil
}

// see if we need to pull a daily update:
//  if we don't have the EOD price for the prior business day
//  OR if we don't have it for the current business day and it's now 7pm or later
func (mi MarketIndex) updateMarketIndexDailies(ctx context.Context) (bool, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)

	mostRecentMarketIndexDaily, err := getMarketIndexDailyMostRecent(db, mi.MarketIndexId)
	if err != nil {
		return false, err
	}
	mostRecentMarketIndexDailyDate := mostRecentMarketIndexDaily.PriceDate
	mostRecentAvailable := mostRecentPricesAvailable()

	logger.Info().
		Str("symbol", mi.MarketIndexSymbol).
		Str("most_recent_daily_date", mostRecentMarketIndexDailyDate).
		Str("most_recent_available", mostRecentAvailable).
		Msg("check if new EOD available for marketindex")

	if mostRecentMarketIndexDailyDate < mostRecentAvailable {
		_, err = fetchMarketIndexes(ctx)
		if err != nil {
			return false, err
		}
		logger.Info().
			Str("symbol", mi.MarketIndexSymbol).
			Int64("marketindex_id", mi.MarketIndexId).
			Msg("Updated MarkeTindex with latest EOD prices")
		return true, nil
	}

	return false, nil
}

// see if we need to pull marketindex intradays for the selected date:
//  if we don't have the marketindex intraday prices for the selected date
//  AND it was a prior business day or today and it's now 7pm or later
func (mi MarketIndex) updateMarketIndexIntradays(ctx context.Context, intradate string) (bool, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)

	if mi.HasIntraday == false {
		log.Warn().
			Str("symbol", mi.MarketIndexSymbol).
			Msg("MarketIndex does not have intraday data")
		return false, nil
	}

	haveIntradayData, err := gotMarketIndexIntradayData(db, mi.MarketIndexId, intradate)
	if err != nil {
		return false, err
	}
	if haveIntradayData {
		return false, nil
	}

	mostRecentAvailable := mostRecentPricesAvailable()

	logger.Info().
		Str("symbol", mi.MarketIndexSymbol).
		Str("intradate", intradate).
		Str("mostRecentAvailable", mostRecentAvailable).
		Msg("check if intraday data available for marketindex")

	if intradate <= mostRecentAvailable {
		err = fetchMarketIndexIntraday(ctx, mi, intradate)
		if err != nil {
			return false, err
		}
		logger.Info().
			Str("symbol", mi.MarketIndexSymbol).
			Int64("marketindex_id", mi.MarketIndexId).
			Str("intradate", intradate).
			Msg("Updated marketindex with intraday prices")
		return true, nil
	}

	return false, nil
}

// misc -----------------------------------------------------------------------

func getMarketIndex(db *sqlx.DB, symbol string) (*MarketIndex, error) {
	var marketIndex MarketIndex
	err := db.QueryRowx("SELECT * FROM marketindex WHERE marketindex_symbol=?", symbol).StructScan(&marketIndex)
	return &marketIndex, err
}

func getMarketIndexById(db *sqlx.DB, marketIndexId int64) (*MarketIndex, error) {
	var marketIndex MarketIndex
	err := db.QueryRowx("SELECT * FROM marketindex WHERE marketindex_id=?", marketIndexId).StructScan(&marketIndex)
	return &marketIndex, err
}

func createMarketIndex(db *sqlx.DB, marketIndex *MarketIndex) (*MarketIndex, error) {
	var insert = "INSERT INTO marketindex SET marketindex_symbol=?, marketindex_mic=?, marketindex_name=?, country_id=?, currency_id=?, marketindex_has_intraday=?, marketindex_has_eod=?"

	res, err := db.Exec(insert, marketIndex.MarketIndexSymbol, marketIndex.MarketIndexMic, marketIndex.MarketIndexName, marketIndex.CountryId, marketIndex.CurrencyId, marketIndex.HasIntraday, marketIndex.HasEOD)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "marketindex").
			Str("symbol", marketIndex.MarketIndexSymbol).
			Int64("marketIndex_id", marketIndex.MarketIndexId).
			Msg("Failed on INSERT")
	}
	marketIndexId, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "marketindex").
			Str("symbol", marketIndex.MarketIndexSymbol).
			Int64("marketIndex_id", marketIndex.MarketIndexId).
			Msg("Failed on LAST_INSERTId")
	}
	return getMarketIndexById(db, marketIndexId)
}

func getOrCreateMarketIndex(db *sqlx.DB, marketIndex *MarketIndex) (*MarketIndex, error) {
	existing, err := getMarketIndex(db, marketIndex.MarketIndexSymbol)
	if err != nil && marketIndex.MarketIndexId == 0 {
		return createMarketIndex(db, marketIndex)
	}
	return existing, err
}

func createOrUpdateMarketIndex(db *sqlx.DB, marketIndex *MarketIndex) (*MarketIndex, error) {
	var update = "UPDATE marketindex SET marketindex_mic=?, marketindex_name=?, country_id=?, currency_id=?, marketindex_has_intraday=?, marketindex_has_eod=? WHERE marketindex_id=?"

	existing, err := getMarketIndex(db, marketIndex.MarketIndexSymbol)
	if err != nil {
		return createMarketIndex(db, marketIndex)
	}

	_, err = db.Exec(update, marketIndex.MarketIndexMic, marketIndex.MarketIndexName, marketIndex.CountryId, marketIndex.CurrencyId, marketIndex.HasIntraday, marketIndex.HasEOD, existing.MarketIndexId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "marketindex").
			Str("symbol", marketIndex.MarketIndexSymbol).
			Int64("marketindex_id", marketIndex.MarketIndexId).
			Msg("Failed on UPDATE")
	}
	return getMarketIndexById(db, existing.MarketIndexId)
}
