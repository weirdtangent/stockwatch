package main

import (
	"github.com/rs/zerolog/log"
)

func getCountry(country_code string) (*Country, error) {
	var country Country
	err := db_session.QueryRowx("SELECT * FROM country WHERE country_code=?", country_code).StructScan(&country)
	return &country, err
}

func getCountryById(country_id int64) (*Country, error) {
	var country Country
	err := db_session.QueryRowx("SELECT * FROM country WHERE country_id=?", country_id).StructScan(&country)
	return &country, err
}

func createCountry(country *Country) (*Country, error) {
	var insert = "INSERT INTO country SET country_code=?, country_name=?"

	res, err := db_session.Exec(insert, country.Country_code, country.Country_name)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed on INSERT")
	}
	country_id, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed on LAST_INSERT_ID")
	}
	return getCountryById(country_id)
}

func getOrCreateCountry(country *Country) (*Country, error) {
	existing, err := getCountry(country.Country_code)
	if err != nil && existing.Country_id == 0 {
		return createCountry(country)
	}
	return existing, err
}

func createOrUpdateCountry(country *Country) (*Country, error) {
	var update = "UPDATE country SET country_code=?, country_name=? WHERE country_id=?"

	existing, err := getCountry(country.Country_code)
	if err != nil {
		return createCountry(country)
	}

	_, err = db_session.Exec(update, country.Country_code, country.Country_name, existing.Country_id)
	if err != nil {
		log.Warn().Err(err).Msg("Failed on UPDATE")
	}
	return getCountryById(existing.Country_id)
}
