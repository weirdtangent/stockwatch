package main

import "github.com/rs/zerolog"

func loadWebWatches(deps *Dependencies, sublog zerolog.Logger, ticker_id uint64) ([]WebWatch, error) {
	db := deps.db

	rows, err := db.Queryx("SELECT target_date,target_price,source_date,source_company,source_name FROM watch LEFT JOIN source USING (source_id) WHERE ticker_id = ? ORDER BY source_date", ticker_id)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed on SELECT")
	}
	defer rows.Close()

	var webWatch WebWatch
	webwatches := make([]WebWatch, 0, 30)
	for rows.Next() {
		err = rows.StructScan(&webWatch)
		if err != nil {
			sublog.Fatal().Err(err).Msg("Error reading result rows")
		}
		webwatches = append(webwatches, webWatch)
	}
	if err := rows.Err(); err != nil {
		sublog.Fatal().Err(err).Msg("Error reading result rows")
	}

	return webwatches, err
}
