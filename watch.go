package main

func loadWebWatches(deps *Dependencies, ticker_id uint64) ([]WebWatch, error) {
	db := deps.db
	sublog := deps.logger

	rows, err := db.Queryx("SELECT target_date,target_price,source_date,source_company,source_name FROM watch LEFT JOIN source USING (source_id) WHERE ticker_id = ? ORDER BY source_date", ticker_id)
	if err != nil {
		sublog.Fatal().Err(err).
			Str("table_name", "watch").
			Msg("failed on SELECT")
	}
	defer rows.Close()

	var webWatch WebWatch
	webwatches := make([]WebWatch, 0, 30)
	for rows.Next() {
		err = rows.StructScan(&webWatch)
		if err != nil {
			sublog.Fatal().Err(err).
				Str("table_name", "watch").
				Msg("Error reading result rows")
		}
		webwatches = append(webwatches, webWatch)
	}
	if err := rows.Err(); err != nil {
		sublog.Fatal().Err(err).
			Str("table_name", "watch").
			Msg("Error reading result rows")
	}

	return webwatches, err
}
