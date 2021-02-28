package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type OAuth struct {
	OAuthId         int64  `db:"oauth_id"`
	OAuthIssuer     string `db:"oauth_issuer"`
	OAuthIssued     int64  `db:"oauth_issued"`
	OAuthExpires    int64  `db:"oauth_expires"`
	OAuthEmail      string `db:"oauth_email"`
	WatcherId       int64  `db:"watcher_id"`
	PictureURL      string `db:"picture_url"`
	OAuthStatus     string `db:"oauth_status"`
	LastUseDatetime string `db:"lastuse_datetime"`
	SessionID       string `db:"session_id"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

func (o OAuth) Delete(db *sqlx.DB, watcherId int64) error {
	var deleteSQL = "DELETE FROM oauth WHERE oauth_id=? AND watcher_id=?"

	_, err := db.Exec(deleteSQL, o.OAuthId, watcherId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "oauth").
			Msg("Failed on DELETE")
	}
	return err
}

func getOAuth(db *sqlx.DB, emailAddress string) (*OAuth, error) {
	var oauth OAuth
	err := db.QueryRowx("SELECT * FROM oauth WHERE oauth_email=?", emailAddress).StructScan(&oauth)
	return &oauth, err
}

func getOAuthById(db *sqlx.DB, oauthId int64) (*OAuth, error) {
	var oauth OAuth
	err := db.QueryRowx("SELECT * FROM oauth WHERE oauth_id=?", oauthId).StructScan(&oauth)
	return &oauth, err
}

func getOAuthByWatcherId(db *sqlx.DB, watcherId int64) (*OAuth, error) {
	var oauth OAuth
	err := db.QueryRowx("SELECT * FROM oauth WHERE watcher_id=?", watcherId).StructScan(&oauth)
	return &oauth, err
}

func createOAuth(db *sqlx.DB, oauth *OAuth) (*OAuth, error) {
	var insert = "INSERT INTO oauth SET oauth_issuer=?, oauth_issued=?, oauth_expires=?, watcher_id=?, oauth_email=?, picture_url=?, session_id=?, lastuse_datetime=current_timestamp()"

	res, err := db.Exec(insert, oauth.OAuthIssuer, oauth.OAuthIssued, oauth.OAuthExpires, oauth.WatcherId, oauth.OAuthEmail, oauth.PictureURL, oauth.SessionID)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "oauth").
			Msg("Failed on INSERT")
	}
	oauthId, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "oauth").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getOAuthById(db, oauthId)
}

func getOrCreateOAuth(db *sqlx.DB, country *Country) (*Country, error) {
	existing, err := getCountry(db, country.CountryCode)
	if err != nil && existing.CountryId == 0 {
		return createCountry(db, country)
	}
	return existing, err
}

func createOrUpdateOAuth(db *sqlx.DB, oauth *OAuth) (*OAuth, error) {
	var update = "UPDATE oauth SET oauth_issuer=?, oauth_issued=?, oauth_expires=?, watcher_id=?, oauth_email=?, picture_url=?, lastuse_datetime=current_timestamp() WHERE oauth_id=?"

	existing, err := getOAuth(db, oauth.OAuthEmail)
	if err != nil {
		return createOAuth(db, oauth)
	}

	_, err = db.Exec(update, oauth.OAuthIssuer, oauth.OAuthIssued, oauth.OAuthExpires, oauth.WatcherId, oauth.OAuthEmail, oauth.PictureURL, existing.OAuthId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "oauth").
			Msg("Failed on UPDATE")
	}
	return getOAuthById(db, existing.OAuthId)
}
