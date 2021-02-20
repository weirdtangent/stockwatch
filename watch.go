package stockwatch

import (
	"graystorm.com/mylog"
)

func loadWebWatches(ticker_id int64) ([]WebWatch, error) {
	rows, err := db_session.Queryx("SELECT target_date,target_price,source_date,source_company,source_name FROM watch LEFT JOIN source USING (source_id) WHERE ticker_id = ? ORDER BY source_date", ticker_id)
	if err != nil {
		mylog.Error.Fatal(err)
	}
	defer rows.Close()

	var webWatch WebWatch
	webwatches := make([]WebWatch, 0, 30)
	for rows.Next() {
		err = rows.StructScan(&webWatch)
		if err != nil {
			mylog.Error.Fatal(err)
		}
		webwatches = append(webwatches, webWatch)
	}
	if err := rows.Err(); err != nil {
		mylog.Error.Fatal(err)
	}

	return webwatches, err
}
