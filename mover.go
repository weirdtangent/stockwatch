package main

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Mover struct {
	MoverId        int64   `db:"mover_id"`
	SourceId       int64   `db:"source_id"`
	TickerId       int64   `db:"ticker_id"`
	MoverDate      string  `db:"mover_date"`
	MoverType      string  `db:"mover_type"`
	LastPrice      float64 `db:"last_price"`
	PriceChange    float64 `db:"price_change"`
	PriceChangePct float64 `db:"price_change_pct"`
	Volume         int64   `db:"volume"`
	CreateDatetime string  `db:"create_datetime"`
	UpdateDatetime string  `db:"update_datetime"`
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

func (m *Mover) getByUniqueKey(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM mover WHERE source_id=? AND ticker_id=? AND mover_date=? AND mover_type=?`,
		m.SourceId, m.TickerId, m.MoverDate, m.MoverType).StructScan(m)
	return err
}

func (m *Mover) createIfNew(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)
	logger := log.Ctx(ctx)

	if m.MoverType == "" {
		logger.Warn().Msg("Refusing to add mover with blank mover type")
		return nil
	}

	err := m.getByUniqueKey(ctx)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO mover SET source_id=?, ticker_id=?, mover_date=?, mover_type=?, last_price=?, price_change=?, price_change_pct=?, volume=?"
	_, err = db.Exec(insert, m.SourceId, m.TickerId, m.MoverDate, m.MoverType, m.LastPrice, m.PriceChange, m.PriceChangePct, m.Volume)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "mover").
			Msg("Failed on INSERT")
	}
	return err
}

func getMovers(ctx context.Context) (*Movers, error) {
	db := ctx.Value("db").(*sqlx.DB)
	logger := log.Ctx(ctx)

	var movers Movers

	latestDateStr, err := getLatestMoversDate(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find most recent movers date")
		return &movers, err
	}

	var mover Mover
	gainers := make([]WebMover, 0)
	losers := make([]WebMover, 0)
	actives := make([]WebMover, 0)

	rows, err := db.Queryx(`SELECT * FROM mover WHERE mover_date=?`, latestDateStr)
	if err != nil {
		logger.Error().Err(err).Str("mover_date", latestDateStr).Msg("Failed to load movers")
		return &movers, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&mover)
		if err != nil {
			logger.Warn().Err(err).
				Str("table_name", "mover").
				Msg("Error reading result rows")
		} else {
			ticker, err := getTickerById(ctx, mover.TickerId)
			if err == nil {
				switch mover.MoverType {
				case "gainer":
					gainers = append(gainers, WebMover{mover, *ticker})
				case "loser":
					losers = append(losers, WebMover{mover, *ticker})
				case "active":
					actives = append(actives, WebMover{mover, *ticker})
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
	db := ctx.Value("db").(*sqlx.DB)
	var dateStr string

	err := db.QueryRowx(`SELECT mover_date FROM mover ORDER BY mover_date DESC LIMIT 1`).Scan(&dateStr)
	return dateStr, err
}
