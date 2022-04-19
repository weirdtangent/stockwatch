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
