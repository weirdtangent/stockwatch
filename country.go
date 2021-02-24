package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func getCountry(db *sqlx.DB, country_code string) (*Country, error) {
	var country Country
	err := db.QueryRowx("SELECT * FROM country WHERE country_code=?", country_code).StructScan(&country)
	return &country, err
}

func getCountryById(db *sqlx.DB, country_id int64) (*Country, error) {
	var country Country
	err := db.QueryRowx("SELECT * FROM country WHERE country_id=?", country_id).StructScan(&country)
	return &country, err
}

func createCountry(db *sqlx.DB, country *Country) (*Country, error) {
	var insert = "INSERT INTO country SET country_code=?, country_name=?"

	res, err := db.Exec(insert, country.Country_code, country.Country_name)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "country").
			Msg("Failed on INSERT")
	}
	country_id, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "country").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getCountryById(db, country_id)
}

func getOrCreateCountry(db *sqlx.DB, country *Country) (*Country, error) {
	existing, err := getCountry(db, country.Country_code)
	if err != nil && existing.Country_id == 0 {
		return createCountry(db, country)
	}
	return existing, err
}

func createOrUpdateCountry(db *sqlx.DB, country *Country) (*Country, error) {
	var update = "UPDATE country SET country_code=?, country_name=? WHERE country_id=?"

	existing, err := getCountry(db, country.Country_code)
	if err != nil {
		return createCountry(db, country)
	}

	_, err = db.Exec(update, country.Country_code, country.Country_name, existing.Country_id)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "country").
			Msg("Failed on UPDATE")
	}
	return getCountryById(db, existing.Country_id)
}
