package main

import (
	"sort"
)

type TickerIntraday struct {
	TickerIntradayId int64   `db:"intraday_id"`
	TickerId         int64   `db:"ticker_id"`
	PriceDate        string  `db:"price_date"`
	LastPrice        float64 `db:"last_price"`
	Volume           float64 `db:"volume"`
	CreateDatetime   string  `db:"create_datetime"`
	UpdateDatetime   string  `db:"update_datetime"`
}

type TickerIntradays struct {
	Moments []TickerIntraday
}

type ByTickerPriceTime TickerIntradays

func (a ByTickerPriceTime) Len() int { return len(a.Moments) }
func (a ByTickerPriceTime) Less(i, j int) bool {
	return a.Moments[i].PriceDate < a.Moments[j].PriceDate
}
func (a ByTickerPriceTime) Swap(i, j int) { a.Moments[i], a.Moments[j] = a.Moments[j], a.Moments[i] }

func (i TickerIntradays) Sort() *TickerIntradays {
	sort.Sort(ByTickerPriceTime(i))
	return &i
}

func (i TickerIntradays) Reverse() *TickerIntradays {
	sort.Sort(sort.Reverse(ByTickerPriceTime(i)))
	return &i
}

func (i TickerIntradays) Count() int {
	return len(i.Moments)
}
