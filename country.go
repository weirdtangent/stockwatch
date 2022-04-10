package main

// func getCountry(db *sqlx.DB, countryCode string) (*Country, error) {
// 	var country Country
// 	err := db.QueryRowx("SELECT * FROM country WHERE country_code=?", countryCode).StructScan(&country)
// 	return &country, err
// }

// func getCountryById(db *sqlx.DB, countryId int64) (*Country, error) {
// 	var country Country
// 	err := db.QueryRowx("SELECT * FROM country WHERE country_id=?", countryId).StructScan(&country)
// 	return &country, err
// }

// func getCountryByName(db *sqlx.DB, countryName string) (*Country, error) {
// 	var country Country
// 	err := db.QueryRowx("SELECT * FROM country WHERE country_name=?", countryName).StructScan(&country)
// 	return &country, err
// }

// func createCountry(db *sqlx.DB, country *Country) (*Country, error) {
// 	var insert = "INSERT INTO country SET country_code=?, country_name=?"

// 	if country.CountryCode == "" {
// 		return country, fmt.Errorf("skipping record with blank country code")
// 	}

// 	res, err := db.Exec(insert, country.CountryCode, country.CountryName)
// 	if err != nil {
// 		log.Fatal().Err(err).
// 			Str("table_name", "country").
// 			Msg("failed on INSERT")
// 	}
// 	countryId, err := res.LastInsertId()
// 	if err != nil {
// 		log.Fatal().Err(err).
// 			Str("table_name", "country").
// 			Msg("failed on LAST_INSERT_ID")
// 	}
// 	return getCountryById(db, countryId)
// }

// func getOrCreateCountry(db *sqlx.DB, country *Country) (*Country, error) {
// 	existing, err := getCountry(db, country.CountryCode)
// 	if err != nil && existing.CountryId == 0 {
// 		return createCountry(db, country)
// 	}
// 	return existing, err
// }

// func createOrUpdateCountry(db *sqlx.DB, country *Country) (*Country, error) {
// 	var update = "UPDATE country SET country_code=?, country_name=? WHERE country_id=?"

// 	if country.CountryCode == "" {
// 		return country, fmt.Errorf("skipping record with blank country code")
// 	}

// 	existing, err := getCountry(db, country.CountryCode)
// 	if err != nil {
// 		return createCountry(db, country)
// 	}

// 	_, err = db.Exec(update, country.CountryCode, country.CountryName, existing.CountryId)
// 	if err != nil {
// 		log.Warn().Err(err).
// 			Str("table_name", "country").
// 			Msg("failed on UPDATE")
// 	}
// 	return getCountryById(db, existing.CountryId)
// }
