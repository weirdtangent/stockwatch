package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Article struct {
	ArticleId          int64  `db:"article_id"`
	SourceId           int64  `db:"source_id"`
	ExternalId         string `db:"external_id"`
	PublishedDatetime  string `db:"published_datetime"`
	PubUpdatedDatetime string `db:"pubupdated_datetime"`
	Title              string `db:"title"`
	Body               string `db:"body"`
	ArticleURL         string `db:"article_url"`
	ImageURL           string `db:"image_url"`
	CreateDatetime     string `db:"create_datetime"`
	UpdateDatetime     string `db:"update_datetime"`
}

// +-------------------+----------+------+-----+-------------------+-----------------------------+
// | Field             | Type     | Null | Key | Default           | Extra                       |
// +-------------------+----------+------+-----+-------------------+-----------------------------+
// | article_ticker_id | int(11)  | NO   | PRI | NULL              | auto_increment              |
// | article_id        | int(11)  | NO   | MUL | NULL              |                             |
// | ticker_symbol     | char(20) | NO   |     | NULL              |                             |
// | ticker_id         | int(11)  | NO   |     | NULL              |                             |
// | create_datetime   | datetime | NO   |     | CURRENT_TIMESTAMP |                             |
// | update_datetime   | datetime | NO   |     | NULL              | on update CURRENT_TIMESTAMP |
// +-------------------+----------+------+-----+-------------------+-----------------------------+
// 6 rows in set (0.00 sec)
//
// +--------------------+----------+------+-----+-------------------+-----------------------------+
// | Field              | Type     | Null | Key | Default           | Extra                       |
// +--------------------+----------+------+-----+-------------------+-----------------------------+
// | article_keyword_id | int(11)  | NO   | PRI | NULL              | auto_increment              |
// | article_id         | int(11)  | NO   | MUL | NULL              |                             |
// | keyword            | char(64) | NO   |     | NULL              |                             |
// | create_datetime    | datetime | NO   |     | CURRENT_TIMESTAMP |                             |
// | update_datetime    | datetime | NO   |     | NULL              | on update CURRENT_TIMESTAMP |
// +--------------------+----------+------+-----+-------------------+-----------------------------+
// 5 rows in set (0.00 sec)
//
type ArticleAuthor struct {
	ArticleAuthorId int64  `db:"article_author_id"`
	ArticleId       int64  `db:"article_id"`
	Byline          string `db:"byline"`
	JobTitle        string `db:"job_title"`
	ShortBio        string `db:"short_bio"`
	LongBio         string `db:"long_bio"`
	ImageURL        string `db:"image_url"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

//
// +-----------------+----------+------+-----+-------------------+-----------------------------+
// | Field           | Type     | Null | Key | Default           | Extra                       |
// +-----------------+----------+------+-----+-------------------+-----------------------------+
// | article_tag_id  | int(11)  | NO   | PRI | NULL              | auto_increment              |
// | article_id      | int(11)  | NO   | MUL | NULL              |                             |
// | tag             | char(64) | NO   |     | NULL              |                             |
// | create_datetime | datetime | NO   |     | CURRENT_TIMESTAMP |                             |
// | update_datetime | datetime | NO   |     | NULL              | on update CURRENT_TIMESTAMP |
// +-----------------+----------+------+-----+-------------------+-----------------------------+
// 5 rows in set (0.00 sec)

type WebArticle struct {
	ArticleId          int64  `db:"article_id"`
	SourceId           int64  `db:"source_id"`
	ExternalId         string `db:"external_id"`
	PublishedDatetime  string `db:"published_datetime"`
	PubUpdatedDatetime string `db:"pubupdated_datetime"`
	Title              string `db:"title"`
	Body               string `db:"body"`
	ArticleURL         string `db:"article_url"`
	ImageURL           string `db:"image_url"`
	CreateDatetime     string `db:"create_datetime"`
	UpdateDatetime     string `db:"update_datetime"`
	Byline             string `db:"byline"`
}

func (a *Article) getArticleById(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM article WHERE article_id=?", a.ArticleId).StructScan(a)
	return err
}

func (a *Article) createArticle(ctx context.Context) error {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)

	var insert = "INSERT INTO article SET source_id=?, external_id=?, published_datetime=?, pubupdated_datetime=?, title=?, body=?, article_url=?, image_url=?"

	res, err := db.Exec(insert, a.SourceId, a.ExternalId, a.PublishedDatetime, a.PubUpdatedDatetime, a.Title, a.Body, a.ArticleURL, a.ImageURL)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "article").
			Msg("Failed on INSERT")
	}
	articleId, err := res.LastInsertId()
	if err != nil || articleId == 0 {
		logger.Fatal().Err(err).
			Str("table_name", "article").
			Msg("Failed on LAST_INSERT_ID")
	}
	a.ArticleId = articleId
	return a.getArticleById(ctx)
}

func getArticleByExternalId(ctx context.Context, sourceId int64, externalId string) (int64, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)

	var articleId int64
	err := db.QueryRowx("SELECT article_id FROM article WHERE source_id=? && external_id=?", sourceId, externalId).Scan(&articleId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		} else {
			logger.Warn().Err(err).Str("table_name", "article").Msg("Failed to check for existing record")
		}
	}
	return articleId, err
}

func getSourceId(source string) (int64, error) {
	if source == "Morningstar" {
		return 2, nil
	} else if source == "Bloomberg" {
		return 3, nil
	}
	return 0, fmt.Errorf("Sorry, unknown source string")
}

func getArticlesByKeyword(ctx context.Context, keyword string) (*[]WebArticle, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)

	var article WebArticle

	fromDate := time.Now().AddDate(0, 0, -2).Format("2006-01-02 15:04:05")

	articles := make([]WebArticle, 0)

	rows, err := db.Queryx(`SELECT article.*,article_author.byline FROM article LEFT JOIN article_author USING (article_id) WHERE published_datetime > ? ORDER BY published_datetime DESC`, fromDate)
	if err != nil {
		return &articles, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&article)
		if err != nil {
			logger.Warn().Err(err).
				Str("table_name", "article,article_author").
				Msg("Error reading result rows")
		} else {
			articles = append(articles, article)
		}
	}
	if err := rows.Err(); err != nil {
		return &articles, err
	}

	return &articles, nil
}

// article authors ------------------------------------------------------------

func (aa *ArticleAuthor) getArticleAuthorById(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM article_author WHERE article_author_id=?", aa.ArticleAuthorId).StructScan(aa)
	return err
}

func (aa *ArticleAuthor) createArticleAuthor(ctx context.Context) error {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)

	var insert = "INSERT INTO article_author SET article_id=?, byline=?, job_title=?, short_bio=?, long_bio=?, image_url=?"

	res, err := db.Exec(insert, aa.ArticleId, aa.Byline, aa.JobTitle, aa.ShortBio, aa.LongBio, aa.ImageURL)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "article_author").
			Msg("Failed on INSERT")
	}
	articleAuthorId, err := res.LastInsertId()
	if err != nil || articleAuthorId == 0 {
		logger.Fatal().Err(err).
			Str("table_name", "article_author").
			Msg("Failed on LAST_INSERT_ID")
	}
	aa.ArticleAuthorId = articleAuthorId
	return aa.getArticleAuthorById(ctx)
}
