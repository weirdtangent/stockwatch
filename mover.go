package main

import (
	"context"
	"database/sql"
	"sort"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Mover struct {
	MoverId        uint64       `db:"mover_id"`
	SourceId       uint64       `db:"source_id"`
	TickerId       uint64       `db:"ticker_id"`
	MoverDate      string       `db:"mover_date"`
	MoverType      string       `db:"mover_type"`
	LastPrice      float64      `db:"last_price"`
	PriceChange    float64      `db:"price_change"`
	PriceChangePct float64      `db:"price_change_pct"`
	Volume         float64      `db:"volume"`
	CreateDatetime sql.NullTime `db:"create_datetime"`
	UpdateDatetime sql.NullTime `db:"update_datetime"`
}

type WebMover struct {
	Mover  Mover
	Ticker Ticker
}

type Movers struct {
	Gainers []WebMover
	Losers  []WebMover
	Actives []WebMover
	ForDate string
}

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

func getMovers(ctx context.Context) (*Movers, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var movers Movers

	latestDateStr, err := getLatestMoversDate(ctx)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to find most recent movers date")
		return &movers, err
	}

	var mover Mover
	gainers := make([]WebMover, 0)
	losers := make([]WebMover, 0)
	actives := make([]WebMover, 0)

	rows, err := db.Queryx(`SELECT * FROM mover WHERE mover_date=?`, latestDateStr)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Str("mover_date", latestDateStr).Msg("Failed to load movers")
		return &movers, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&mover)
		if err != nil {
			log.Warn().Err(err).Str("table_name", "mover").Msg("Error reading result rows")
		} else {
			ticker := Ticker{TickerId: mover.TickerId}
			err := ticker.getById(ctx)
			if err == nil {
				switch mover.MoverType {
				case "gainer":
					gainers = append(gainers, WebMover{mover, ticker})
				case "loser":
					losers = append(losers, WebMover{mover, ticker})
				case "active":
					actives = append(actives, WebMover{mover, ticker})
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return &movers, err
	}

	movers = Movers{gainers, losers, actives, latestDateStr}
	return &movers, nil
}

func getLatestMoversDate(ctx context.Context) (string, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	var dateStr string

	err := db.QueryRowx(`SELECT mover_date FROM mover ORDER BY mover_date DESC LIMIT 1`).Scan(&dateStr)
	return dateStr, err
}
