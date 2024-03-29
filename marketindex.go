package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog"
	"github.com/weirdtangent/mytime"
)

type MarketIndex struct {
	MarketIndexId     uint64 `db:"marketindex_id"`
	EId               string
	MarketIndexSymbol string    `db:"marketindex_symbol"`
	MarketIndexMic    string    `db:"marketindex_mic"`
	MarketIndexName   string    `db:"marketindex_name"`
	CountryId         uint64    `db:"country_id"`
	HasIntraday       bool      `db:"marketindex_has_intraday"`
	HasEOD            bool      `db:"marketindex_has_eod"`
	CurrencyId        uint64    `db:"currency_id"`
	CreateDatetime    time.Time `db:"create_datetime"`
	UpdateDatetime    time.Time `db:"update_datetime"`
}

type MarketIndexDaily struct {
	MarketIndexDailyId uint64 `db:"marketindex_daily_id"`
	EId                string
	MarketIndexId      uint64    `db:"marketindex_id"`
	PriceDate          string    `db:"price_date"`
	OpenPrice          float64   `db:"open_price"`
	HighPrice          float64   `db:"high_price"`
	LowPrice           float64   `db:"low_price"`
	ClosePrice         float64   `db:"close_price"`
	Volume             int64     `db:"volume"`
	CreateDatetime     time.Time `db:"create_datetime"`
	UpdateDatetime     time.Time `db:"update_datetime"`
}

type MarketIndexDailies struct {
	Days []MarketIndexDaily
}

type MarketIndexByPriceDate MarketIndexDailies

type MarketIndexIntraday struct {
	MarketIndexIntradayId uint64 `db:"intraday_id"`
	EId                   string
	TickerId              uint64    `db:"ticker_id"`
	PriceDate             string    `db:"price_date"`
	LastPrice             float64   `db:"last_price"`
	Volume                int64     `db:"volume"`
	CreateDatetime        time.Time `db:"create_datetime"`
	UpdateDatetime        time.Time `db:"update_datetime"`
}

type MarketIndexIntradays struct {
	Moments []MarketIndexIntraday
}

type ByMarketIndexPriceTime MarketIndexIntradays

// object methods -------------------------------------------------------------

func (mi MarketIndex) LoadDailies(deps *Dependencies, sublog zerolog.Logger, days int) ([]MarketIndexDaily, error) {
	db := deps.db

	daily := MarketIndexDaily{}
	dailies := make([]MarketIndexDaily, 0, days)

	rows, err := db.Queryx(
		`SELECT * FROM (
       SELECT * FROM marketindex_daily WHERE marketindex_id=? AND volume > 0
         ORDER BY price_datetime DESC LIMIT ?) DT1
     ORDER BY price_date`,
		mi.MarketIndexId, days)
	if err != nil {
		return dailies, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&daily)
		if err != nil {
			sublog.Warn().Err(err).Msg("failed reading result rows")
		} else {
			dailies = append(dailies, daily)
		}
	}
	if err := rows.Err(); err != nil {
		return dailies, err
	}

	return dailies, nil
}

func (mi MarketIndex) LoadMarketIndexIntraday(deps *Dependencies, intradate string) ([]MarketIndexIntraday, error) {
	db := deps.db
	sublog := deps.logger

	var marketindex_intraday MarketIndexIntraday
	marketindex_intradays := make([]MarketIndexIntraday, 0)

	rows, err := db.Queryx(
		`SELECT * FROM marketindex_intraday
                 WHERE marketindex_id=? AND price_dateitime LIKE ? AND volume > 0
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
	preDaily, err := getMarketIndexDaily(deps, *sublog, mi.MarketIndexId, priorBusinessDay)
	if err == nil {
		marketindex_intradays = append(marketindex_intradays, MarketIndexIntraday{0, "", mi.MarketIndexId, priorBusinessDay, preDaily.ClosePrice, 0, time.Now(), time.Now()})
	} else {
		sublog.Info().Msg("failure: PriorBusinessDay close price was NOT included")
	}

	// add these marketindex intraday prices
	for rows.Next() {
		err = rows.StructScan(&marketindex_intraday)
		if err != nil {
			sublog.Warn().Err(err).Msg("failed reading result rows")
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
	postDaily, err := getMarketIndexDaily(deps, *sublog, mi.MarketIndexId, nextBusinessDay)
	if err == nil {
		marketindex_intradays = append(marketindex_intradays, MarketIndexIntraday{0, "", mi.MarketIndexId, nextBusinessDay, postDaily.OpenPrice, 0, time.Now(), time.Now()})
	} else {
		sublog.Info().Msg("failure: NextBusinessDay open price was NOT included")
	}

	return marketindex_intradays, nil
}

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

// misc -----------------------------------------------------------------------

func getMarketIndexDaily(deps *Dependencies, sublog zerolog.Logger, marketindex_id uint64, daily_date string) (*MarketIndexDaily, error) {
	db := deps.db

	marketindexdaily := MarketIndexDaily{}
	if len(daily_date) > 10 {
		daily_date = daily_date[0:10]
	}
	err := db.QueryRowx(
		`SELECT * FROM marketindex_daily WHERE marketindex_id=? AND price_date=?`,
		marketindex_id, daily_date).StructScan(&marketindexdaily)
	return &marketindexdaily, err
}
