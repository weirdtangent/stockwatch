package main

import (
	"github.com/rs/zerolog/log"
)

func loadWebWatches(ticker_id int64) ([]WebWatch, error) {
	rows, err := db_session.Queryx("SELECT target_date,target_price,source_date,source_company,source_name FROM watch LEFT JOIN source USING (source_id) WHERE ticker_id = ? ORDER BY source_date", ticker_id)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed on SELECT")
	}
	defer rows.Close()

	var webWatch WebWatch
	webwatches := make([]WebWatch, 0, 30)
	for rows.Next() {
		err = rows.StructScan(&webWatch)
		if err != nil {
			log.Fatal().Err(err).Msg("Error reading result rows")
		}
		webwatches = append(webwatches, webWatch)
	}
	if err := rows.Err(); err != nil {
		log.Fatal().Err(err).Msg("Error reading result rows")
	}

	return webwatches, err
}
