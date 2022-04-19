package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"html/template"
	"regexp"
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

type ArticleTicker struct {
	ArticleTickerId int64  `db:"article_ticker_id"`
	ArticleId       int64  `db:"article_id"`
	TickerSymbol    string `db:"ticker_symbol"`
	TickerId        int64  `db:"ticker_id"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

type ArticleKeyword struct {
	ArticleKeywordId int64  `db:"article_keyword_id"`
	ArticleId        int64  `db:"article_id"`
	Keyword          string `db:"keyword"`
	CreateDatetime   string `db:"create_datetime"`
	UpdateDatetime   string `db:"update_datetime"`
}

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

type ArticleTag struct {
	ArticleTagId   int64  `db:"article_tag_id"`
	ArticleId      int64  `db:"article_id"`
	Tag            string `db:"tag"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

type WebArticle struct {
	ArticleId          int64  `db:"article_id"`
	SourceId           int64  `db:"source_id"`
	ExternalId         string `db:"external_id"`
	PublishedDatetime  string `db:"published_datetime"`
	PubUpdatedDatetime string `db:"pubupdated_datetime"`
	Title              string `db:"title"`
	Body               string `db:"body"`
	BodyTemplate       template.HTML
	ArticleURL         string         `db:"article_url"`
	ImageURL           string         `db:"image_url"`
	CreateDatetime     string         `db:"create_datetime"`
	UpdateDatetime     string         `db:"update_datetime"`
	AuthorByline       sql.NullString `db:"author_byline"`
	AuthorLongBio      sql.NullString `db:"author_long_bio"`
	AuthorImageURL     sql.NullString `db:"author_image_url"`
	SourceName         sql.NullString `db:"source_name"`
	Keywords           sql.NullString `db:"keywords"`
	Tags               sql.NullString `db:"tags"`
	Symbols            sql.NullString `db:"symbols"`
}

func getArticlesByKeyword(ctx context.Context, keyword string) (*[]WebArticle, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var article WebArticle

	var query string
	var fromDate string
	var rows *sqlx.Rows
	var err error

	articles := make([]WebArticle, 0)

	if len(keyword) > 0 {
		fromDate = time.Now().AddDate(0, 0, -120).Format("2006-01-02 15:04:05")
		query = `SELECT article.article_id, article.source_id, article.external_id, article.published_datetime, article.pubupdated_datetime,
		            article.title, article.body, article.article_url, article.image_url,
                    ANY_VALUE(article_author.byline) AS author_byline,
					ANY_VALUE(article_author.long_bio) AS author_long_bio,
					ANY_VALUE(article_author.image_url) AS author_image_url,
					source.source_name AS source_name,
					GROUP_CONCAT(DISTINCT article_keyword.keyword ORDER BY article_keyword.keyword SEPARATOR ', ') AS keywords,
					GROUP_CONCAT(DISTINCT article_tag.tag ORDER BY article_tag.tag SEPARATOR ', ') AS tags,
					GROUP_CONCAT(DISTINCT article_ticker.ticker_symbol ORDER BY article_ticker.ticker_symbol SEPARATOR ', ') AS symbols
				FROM article
 				LEFT JOIN article_author USING (article_id)
				LEFT JOIN article_ticker USING (article_id)
				LEFT JOIN article_keyword USING (article_id)
				LEFT JOIN article_tag USING (article_id)
				LEFT JOIN source USING (source_id)
				WHERE published_datetime > ?
					AND (keyword=? OR tag=? OR ticker_symbol=?)
  				GROUP BY article_id
				ORDER BY published_datetime DESC
    			LIMIT 6`
		rows, err = db.Queryx(query, fromDate, keyword, keyword, keyword)
	} else {
		fromDate := time.Now().AddDate(0, 0, -5).Format("2006-01-02 15:04:05")
		query = `SELECT article.article_id, article.source_id, article.external_id, article.published_datetime, article.pubupdated_datetime,
		            article.title, article.body, article.article_url, article.image_url,
                	ANY_VALUE(article_author.byline) AS author_byline,
					ANY_VALUE(article_author.long_bio) AS author_long_bio,
					ANY_VALUE(article_author.image_url) AS author_image_url,
					source.source_name AS source_name
				FROM article
				LEFT JOIN article_author USING (article_id)
				LEFT JOIN source USING (source_id)
				WHERE published_datetime > ?
				GROUP BY article_id
				ORDER BY published_datetime DESC
				LIMIT 15`
		rows, err = db.Queryx(query, fromDate)
	}

	if err != nil {
		logger.Warn().Err(err).Msg("Failed to check for articles")
		return &articles, err
	}
	defer rows.Close()

	bodySHA256 := make(map[string]bool)

	for rows.Next() {
		err = rows.StructScan(&article)
		if err != nil {
			logger.Warn().Err(err).
				Str("table_name", "article,article_author").
				Msg("Error reading result rows")
		} else {
			if len(article.Body) > 0 {
				sha := fmt.Sprintf("%x", sha256.Sum256([]byte(article.Body)))
				if _, ok := bodySHA256[sha]; ok {
					log.Info().Msg("Skipping, seen this article body already")
				} else {
					bodySHA256[sha] = true

					quote_rx := regexp.MustCompile(`'`)
					article.Body = string(quote_rx.ReplaceAll([]byte(article.Body), []byte("&apos;")))

					http_rx := regexp.MustCompile(`http:`)
					article.Body = string(http_rx.ReplaceAll([]byte(article.Body), []byte("https:")))
					article.AuthorImageURL.String = string(http_rx.ReplaceAll([]byte(article.AuthorImageURL.String), []byte("https:")))

					articles = append(articles, article)
				}
			} else {
				articles = append(articles, article)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return &articles, err
	}

	return &articles, nil
}

func getArticlesByTicker(ctx context.Context, ticker_id int64) (*[]WebArticle, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var article WebArticle

	var query string
	var fromDate string
	var rows *sqlx.Rows
	var err error

	articles := make([]WebArticle, 0)

	// go back as far as 180 days
	fromDate = time.Now().AddDate(0, 0, -180).Format("2006-01-02 15:04:05")

	query = `SELECT article.article_id, article.source_id, article.external_id, article.published_datetime, article.pubupdated_datetime,
		            article.title, article.body, article.article_url, article.image_url,
                    ANY_VALUE(article_author.byline) AS author_byline,
					ANY_VALUE(article_author.long_bio) AS author_long_bio,
					ANY_VALUE(article_author.image_url) AS author_image_url,
					source.source_name AS source_name,
					GROUP_CONCAT(DISTINCT article_keyword.keyword ORDER BY article_keyword.keyword SEPARATOR ', ') AS keywords,
					GROUP_CONCAT(DISTINCT article_tag.tag ORDER BY article_tag.tag SEPARATOR ', ') AS tags,
					GROUP_CONCAT(DISTINCT article_ticker.ticker_symbol ORDER BY article_ticker.ticker_symbol SEPARATOR ', ') AS symbols
				FROM article
 				LEFT JOIN article_author USING (article_id)
				LEFT JOIN article_ticker USING (article_id)
				LEFT JOIN article_keyword USING (article_id)
				LEFT JOIN article_tag USING (article_id)
				LEFT JOIN source USING (source_id)
				WHERE published_datetime > ?
					AND ticker_id=?
  				GROUP BY article_id
				ORDER BY published_datetime DESC
    			LIMIT 20`
	rows, err = db.Queryx(query, fromDate, ticker_id)

	if err != nil {
		logger.Warn().Err(err).Msg("Failed to check for articles")
		return &articles, err
	}
	defer rows.Close()

	bodySHA256 := make(map[string]bool)

	for rows.Next() {
		err = rows.StructScan(&article)
		if err != nil {
			logger.Warn().Err(err).
				Str("table_name", "article,article_author").
				Msg("Error reading result rows")
		} else {
			if len(article.Body) > 0 {
				sha := fmt.Sprintf("%x", sha256.Sum256([]byte(article.Body)))
				if _, ok := bodySHA256[sha]; ok {
					log.Info().Msg("Skipping, seen this article body already")
				} else {
					bodySHA256[sha] = true

					quote_rx := regexp.MustCompile(`'`)
					article.Body = string(quote_rx.ReplaceAll([]byte(article.Body), []byte("&apos;")))

					http_rx := regexp.MustCompile(`http:`)
					article.Body = string(http_rx.ReplaceAll([]byte(article.Body), []byte("https:")))
					article.AuthorImageURL.String = string(http_rx.ReplaceAll([]byte(article.AuthorImageURL.String), []byte("https:")))

					articles = append(articles, article)
				}
			} else {
				articles = append(articles, article)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return &articles, err
	}

	return &articles, nil
}
