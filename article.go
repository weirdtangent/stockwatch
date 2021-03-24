package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/mytime"
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
// +-------------------+--------------+------+-----+-------------------+-----------------------------+
// | Field             | Type         | Null | Key | Default           | Extra                       |
// +-------------------+--------------+------+-----+-------------------+-----------------------------+
// | article_author_id | int(11)      | NO   | PRI | NULL              | auto_increment              |
// | article_id        | int(11)      | NO   | MUL | NULL              |                             |
// | byline            | varchar(128) | NO   |     | NULL              |                             |
// | job_title         | varchar(128) | NO   |     | NULL              |                             |
// | short_bio         | varchar(255) | NO   |     | NULL              |                             |
// | long_bio          | text         | NO   |     | NULL              |                             |
// | image_url         | varchar(128) | NO   |     | NULL              |                             |
// | create_datetime   | datetime     | NO   |     | CURRENT_TIMESTAMP |                             |
// | update_datetime   | datetime     | NO   |     | NULL              | on update CURRENT_TIMESTAMP |
// +-------------------+--------------+------+-----+-------------------+-----------------------------+
// 9 rows in set (0.00 sec)
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

func getArticlesByKeyword(ctx context.Context, keyword string) (*[]Article, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)

	var article Article

	fromDate := mytime.DateStr(-2)
	articles := make([]Article, 0)

	rows, err := db.Queryx(`SELECT * FROM article WHERE published_datetime > ? ORDER BY published_datetime DESC`, fromDate)
	if err != nil {
		return &articles, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&article)
		if err != nil {
			logger.Warn().Err(err).
				Str("table_name", "article").
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
