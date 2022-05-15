package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog"
)

type Mover struct {
	MoverId        uint64 `db:"mover_id"`
	EId            string
	SourceId       uint64    `db:"source_id"`
	TickerId       uint64    `db:"ticker_id"`
	MoverDate      time.Time `db:"mover_date"`
	MoverType      string    `db:"mover_type"`
	LastPrice      float64   `db:"last_price"`
	PriceChange    float32   `db:"price_change"`
	PriceChangePct float32   `db:"price_change_pct"`
	Volume         int64     `db:"volume"`
	VolumeStr      string
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}

type WebMover struct {
	Mover  Mover
	Ticker Ticker
}

type Movers struct {
	Gainers []WebMover
	Losers  []WebMover
	Actives []WebMover
	ForDate time.Time
}

// object methods -------------------------------------------------------------

type ByGainers Movers

func (a ByGainers) Len() int { return len(a.Gainers) }
func (a ByGainers) Less(i, j int) bool {
	return a.Gainers[i].Mover.PriceChangePct < a.Gainers[j].Mover.PriceChangePct
}
func (a ByGainers) Swap(i, j int) { a.Gainers[i], a.Gainers[j] = a.Gainers[j], a.Gainers[i] }

type ByLosers Movers

func (a ByLosers) Len() int { return len(a.Losers) }
func (a ByLosers) Less(i, j int) bool {
	return a.Losers[i].Mover.PriceChangePct < a.Losers[j].Mover.PriceChangePct
}
func (a ByLosers) Swap(i, j int) { a.Losers[i], a.Losers[j] = a.Losers[j], a.Losers[i] }

type ByActives Movers

func (a ByActives) Len() int { return len(a.Actives) }
func (a ByActives) Less(i, j int) bool {
	return a.Actives[i].Mover.Volume < a.Actives[j].Mover.Volume
}
func (a ByActives) Swap(i, j int) { a.Actives[i], a.Actives[j] = a.Actives[j], a.Actives[i] }

func (m Movers) SortGainers() *[]WebMover {
	sort.Sort(sort.Reverse(ByGainers(m)))
	return &m.Gainers
}

func (m Movers) SortLosers() *[]WebMover {
	sort.Sort(ByLosers(m))
	return &m.Losers
}

func (m Movers) SortActives() *[]WebMover {
	sort.Sort(sort.Reverse(ByActives(m)))
	return &m.Actives
}

// misc -----------------------------------------------------------------------

func getMovers(deps *Dependencies, sublog zerolog.Logger) Movers {
	db := deps.db

	movers := Movers{}
	gainers := make([]WebMover, 0)
	losers := make([]WebMover, 0)
	actives := make([]WebMover, 0)

	latestMoverDate, err := getLatestMoversDate(deps)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to get latest movers date")
		return movers
	}
	sublog = sublog.With().Str("mover_date", latestMoverDate.Format("2006-01-02")).Logger()

	rows, err := db.Queryx(`SELECT * FROM mover WHERE mover_date=?`, latestMoverDate.Format("2006-01-02"))
	if err != nil {
		sublog.Error().Err(err).Msg("failed to load movers")
		return movers
	}

	defer rows.Close()
	mover := Mover{}
	for rows.Next() {
		err = rows.StructScan(&mover)
		if err != nil {
			sublog.Warn().Err(err).Msg("failed reading row")
			continue
		}
		if mover.Volume > 1_000_000 {
			mover.VolumeStr = fmt.Sprintf("%.2fM", float32(mover.Volume)/1_000_000)
		} else if mover.Volume > 1_000 {
			mover.VolumeStr = fmt.Sprintf("%.2fK", float32(mover.Volume)/1_000)
		}
		ticker := Ticker{TickerId: mover.TickerId}
		err := ticker.getById(deps, sublog)
		if err != nil {
			sublog.Warn().Err(err).Msg("failed reading row")
			continue
		}
		switch mover.MoverType {
		case "gainer":
			if len(gainers) < 10 {
				gainers = append(gainers, WebMover{mover, ticker})
			}
		case "loser":
			if len(losers) < 10 {
				losers = append(losers, WebMover{mover, ticker})
			}
		case "active":
			if len(actives) < 10 {
				actives = append(actives, WebMover{mover, ticker})
			}
		}
	}
	if err := rows.Err(); err != nil {
		sublog.Warn().Err(err).Msg("failed reading rows")
		return movers
	}

	movers = Movers{gainers, losers, actives, latestMoverDate}
	return movers
}

func getLatestMoversDate(deps *Dependencies) (time.Time, error) {
	db := deps.db

	maxMoverDate := time.Time{}
	err := db.QueryRowx(`SELECT MAX(mover_date) FROM mover`).Scan(&maxMoverDate)
	return maxMoverDate, err
}
