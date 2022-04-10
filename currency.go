package main

type Currency struct {
	CurrencyId           int64  `db:"currency_id"`
	CurrencyCode         string `db:"currency_code"`
	CurrencyName         string `db:"currency_name"`
	CurrencySymbol       string `db:"currency_symbol"`
	CurrencySymbolNative string `db:"currency_symbol_native"`
	CreateDatetime       string `db:"create_datetime"`
	UpdateDatetime       string `db:"update_datetime"`
}

// func getCurrency(db *sqlx.DB, currencyCode string) (*Currency, error) {
// 	var currency Currency
// 	err := db.QueryRowx("SELECT * FROM currency WHERE currency_code=?", currencyCode).StructScan(&currency)
// 	return &currency, err
// }

// func getCurrencyById(db *sqlx.DB, currencyId int64) (*Currency, error) {
// 	var currency Currency
// 	err := db.QueryRowx("SELECT * FROM currency WHERE currency_id=?", currencyId).StructScan(&currency)
// 	return &currency, err
// }

// func createCurrency(db *sqlx.DB, currency *Currency) (*Currency, error) {
// 	var insert = "INSERT INTO currency SET currency_code=?, currency_name=?, currency_symbol=?, currency_symbol_native=?"

// 	if currency.CurrencyCode == "" {
// 		return currency, fmt.Errorf("skipping record with blank currency code")
// 	}

// 	res, err := db.Exec(insert, currency.CurrencyCode, currency.CurrencyName, currency.CurrencySymbol, currency.CurrencySymbolNative)
// 	if err != nil {
// 		log.Fatal().Err(err).
// 			Str("table_name", "currency").
// 			Msg("failed on INSERT")
// 	}
// 	currencyId, err := res.LastInsertId()
// 	if err != nil {
// 		log.Fatal().Err(err).
// 			Str("table_name", "currency").
// 			Msg("failed on LAST_INSERT_ID")
// 	}
// 	return getCurrencyById(db, currencyId)
// }

// func getOrCreateCurrency(db *sqlx.DB, currency *Currency) (*Currency, error) {
// 	existing, err := getCurrency(db, currency.CurrencyCode)
// 	if err != nil && existing.CurrencyId == 0 {
// 		return createCurrency(db, currency)
// 	}
// 	return existing, err
// }

// func createOrUpdateCurrency(db *sqlx.DB, currency *Currency) (*Currency, error) {
// 	var update = "UPDATE currency SET currency_code=?, currency_name=?, currency_symbol=?, currency_symbol_native=? WHERE currency_id=?"

// 	if currency.CurrencyCode == "" {
// 		return currency, fmt.Errorf("skipping record with blank currency code")
// 	}

// 	existing, err := getCurrency(db, currency.CurrencyCode)
// 	if err != nil {
// 		return createCurrency(db, currency)
// 	}

// 	// don't overwrite data with ""
// 	if currency.CurrencySymbolNative == "" {
// 		currency.CurrencySymbolNative = existing.CurrencySymbolNative
// 	}

// 	_, err = db.Exec(update, currency.CurrencyCode, currency.CurrencyName, currency.CurrencySymbol, currency.CurrencySymbolNative, existing.CurrencyId)
// 	if err != nil {
// 		log.Warn().Err(err).
// 			Str("table_name", "currency").
// 			Msg("failed on UPDATE")
// 	}
// 	return getCurrencyById(db, existing.CurrencyId)
// }
