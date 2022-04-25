package main

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

func loadWebWatches(ctx context.Context, ticker_id uint64) ([]WebWatch, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	rows, err := db.Queryx("SELECT target_date,target_price,source_date,source_company,source_name FROM watch LEFT JOIN source USING (source_id) WHERE ticker_id = ? ORDER BY source_date", ticker_id)
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).
			Str("table_name", "watch").
			Msg("Failed on SELECT")
	}
	defer rows.Close()

	var webWatch WebWatch
	webwatches := make([]WebWatch, 0, 30)
	for rows.Next() {
		err = rows.StructScan(&webWatch)
		if err != nil {
			zerolog.Ctx(ctx).Fatal().Err(err).
				Str("table_name", "watch").
				Msg("Error reading result rows")
		}
		webwatches = append(webwatches, webWatch)
	}
	if err := rows.Err(); err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).
			Str("table_name", "watch").
			Msg("Error reading result rows")
	}

	return webwatches, err
}
