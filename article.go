package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Article struct {
	ArticleId      int64  `db:"article_id"`
	SubmitterId    int64  `db:"submitter_id"`
	LinkTitle      string `db:"link_title"`
	LinkDesc       string `db:"link_desc"`
	LinkURL        string `db:"link_url"`
	WatchId        int64  `db:"watch_id"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

func getArticleByURL(db *sqlx.DB, linkURL string) (*Article, error) {
	var article Article
	err := db.QueryRowx("SELECT * FROM article WHERE link_url=?", linkURL).StructScan(&article)
	return &article, err
}

func getArticleById(db *sqlx.DB, articleId int64) (*Article, error) {
	var article Article
	err := db.QueryRowx("SELECT * FROM article WHERE article_id=?", articleId).StructScan(&article)
	return &article, err
}

func createArticle(db *sqlx.DB, article *Article) (*Article, error) {
	var insert = "INSERT INTO article SET submitter_id=?, link_title=?, link_desc=?, link_url=?, watch_id=?"

	res, err := db.Exec(insert, article.SubmitterId, article.LinkTitle, article.LinkDesc, article.LinkURL, article.WatchId)
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "article").
			Msg("Failed on INSERT")
	}
	articleId, err := res.LastInsertId()
	if err != nil {
		log.Fatal().Err(err).
			Str("table_name", "article").
			Msg("Failed on LAST_INSERT_ID")
	}
	return getArticleById(db, articleId)
}

func getOrCreateArticle(db *sqlx.DB, article *Article) (*Article, error) {
	existing, err := getArticleByURL(db, article.LinkURL)
	if err != nil && existing.ArticleId == 0 {
		return createArticle(db, article)
	}
	return existing, err
}

func createOrUpdateArticle(db *sqlx.DB, article *Article) (*Article, error) {
	var update = "UPDATE article SET submitter_id=?, link_title=?, link_desc=?, link_url=?, watch_id=? WHERE article_id=?"

	existing, err := getArticleByURL(db, article.LinkURL)
	if err != nil {
		return createArticle(db, article)
	}

	_, err = db.Exec(update, article.SubmitterId, article.LinkTitle, article.LinkDesc, article.LinkURL, article.WatchId, existing.ArticleId)
	if err != nil {
		log.Warn().Err(err).
			Str("table_name", "article").
			Msg("Failed on UPDATE")
	}
	return getArticleById(db, existing.ArticleId)
}
