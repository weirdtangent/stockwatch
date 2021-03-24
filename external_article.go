package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type ExternalArticle struct {
	ExternalArticleId int64  `db:"external_article_id"`
	SubmitterId       int64  `db:"submitter_id"`
	LinkTitle         string `db:"link_title"`
	LinkDesc          string `db:"link_desc"`
	LinkURL           string `db:"link_url"`
	WatchId           int64  `db:"watch_id"`
	CreateDatetime    string `db:"create_datetime"`
	UpdateDatetime    string `db:"update_datetime"`
}

func getExternalArticleByURL(db *sqlx.DB, linkURL string) (*ExternalArticle, error) {
	var externalArticle ExternalArticle
	err := db.QueryRowx("SELECT * FROM external_article WHERE link_url=?", linkURL).StructScan(&externalArticle)
	return &externalArticle, err
}

func getExternalArticleById(db *sqlx.DB, externalArticleId int64) (*ExternalArticle, error) {
	var externalArticle ExternalArticle
	err := db.QueryRowx("SELECT * FROM external_article WHERE external_article_id=?", externalArticleId).StructScan(&externalArticle)
	return &externalArticle, err
}

func createExternalArticle(db *sqlx.DB, externalArticle *ExternalArticle) (*ExternalArticle, error) {
	var insert = "INSERT INTO external_article SET submitter_id=?, link_title=?, link_desc=?, link_url=?, watch_id=?"

	res, err := db.Exec(insert, externalArticle.SubmitterId, externalArticle.LinkTitle, externalArticle.LinkDesc, externalArticle.LinkURL, externalArticle.WatchId)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "external_article").
			Msg("Failed on INSERT")
	}
	externalArticleId, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "external_article").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getExternalArticleById(db, externalArticleId)
}

func getOrCreateExternalArticle(db *sqlx.DB, externalArticle *ExternalArticle) (*ExternalArticle, error) {
	existing, err := getExternalArticleByURL(db, externalArticle.LinkURL)
	if err != nil && existing.ExternalArticleId == 0 {
		return createExternalArticle(db, externalArticle)
	}
	return existing, err
}

func createOrUpdateExternalArticle(db *sqlx.DB, externalArticle *ExternalArticle) (*ExternalArticle, error) {
	var update = "UPDATE external_article SET submitter_id=?, link_title=?, link_desc=?, link_url=?, watch_id=? WHERE external_article_id=?"

	existing, err := getExternalArticleByURL(db, externalArticle.LinkURL)
	if err != nil {
		return createExternalArticle(db, externalArticle)
	}

	_, err = db.Exec(update, externalArticle.SubmitterId, externalArticle.LinkTitle, externalArticle.LinkDesc, externalArticle.LinkURL, externalArticle.WatchId, existing.ExternalArticleId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "external_article").
			Msg("Failed on UPDATE")
	}
	return getExternalArticleById(db, existing.ExternalArticleId)
}
