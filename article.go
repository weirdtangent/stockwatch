package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
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
	ArticleURL         string `db:"article_url"`
	ImageURL           string `db:"image_url"`
	CreateDatetime     string `db:"create_datetime"`
	UpdateDatetime     string `db:"update_datetime"`
	AuthorByline       string `db:"author_byline"`
	AuthorLongBio      string `db:"author_long_bio"`
	AuthorImageURL     string `db:"author_image_url"`
	SourceName         string `db:"source_name"`
	Keywords           string `db:"keywords"`
	Tags               string `db:"tags"`
	Symbols            string `db:"symbols"`
}

func (a *Article) getArticleById(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM article WHERE article_id=?", a.ArticleId).StructScan(a)
	return err
}

func (a *Article) createArticle(ctx context.Context) error {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

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
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

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
	return 0, fmt.Errorf("unknown source string")
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
		log.Info().Str("from_date", fromDate).Str("keyword", keyword).Int("limit", 6).Msg("Pulling recent articles by keyword")
		query = `SELECT article.*,
                    article_author.byline AS author_byline,
										article_author.long_bio AS author_long_bio,
										article_author.image_url AS author_image_url,
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
		log.Info().Str("from_date", fromDate).Msg("Pulling all articles by date")
		query = `SELECT article.*,
                    article_author.byline AS author_byline,
										article_author.long_bio AS author_long_bio,
										article_author.image_url AS author_image_url,
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
					article.AuthorImageURL = string(http_rx.ReplaceAll([]byte(article.AuthorImageURL), []byte("https:")))

					log.Info().Str("body", sha).Msg("Selected article to show")
					articles = append(articles, article)
				}
			} else {
				log.Info().Str("body", "-empty-").Msg("Selected article to show")
				articles = append(articles, article)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return &articles, err
	}

	return &articles, nil
}

// article authors ------------------------------------------------------------

func (aa *ArticleAuthor) getArticleAuthorById(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM article_author WHERE article_author_id=?", aa.ArticleAuthorId).StructScan(aa)
	return err
}

func (aa *ArticleAuthor) createArticleAuthor(ctx context.Context) error {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

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

// article tickers ------------------------------------------------------------

// func (at *ArticleTicker) getArticleTickerById(ctx context.Context) error {
// 	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

// 	err := db.QueryRowx("SELECT * FROM article_ticker WHERE article_ticker_id=?", at.ArticleTickerId).StructScan(at)
// 	return err
// }

// func (at *ArticleTicker) createArticleTicker(ctx context.Context) error {
// 	logger := log.Ctx(ctx)
// 	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

// 	var insert = "INSERT INTO article_ticker SET article_id=?, ticker_symbol=?, ticker_id=?"

// 	res, err := db.Exec(insert, at.ArticleId, at.TickerSymbol, at.TickerId)
// 	if err != nil {
// 		logger.Fatal().Err(err).
// 			Str("table_name", "article_ticker").
// 			Msg("Failed on INSERT")
// 	}
// 	articleTickerId, err := res.LastInsertId()
// 	if err != nil || articleTickerId == 0 {
// 		logger.Fatal().Err(err).
// 			Str("table_name", "article_ticker").
// 			Msg("Failed on LAST_INSERT_ID")
// 	}
// 	at.ArticleTickerId = articleTickerId
// 	return at.getArticleTickerById(ctx)
// }

// // article keywords -----------------------------------------------------------

// func (ak *ArticleKeyword) createArticleKeyword(ctx context.Context) error {
// 	logger := log.Ctx(ctx)
// 	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

// 	var insert = "INSERT INTO article_keyword SET article_id=?, keyword=?"

// 	_, err := db.Exec(insert, ak.ArticleId, ak.Keyword)
// 	if err != nil {
// 		logger.Fatal().Err(err).
// 			Str("table_name", "article_keyword").
// 			Msg("Failed on INSERT")
// 	}
// 	return err
// }

// // article tags -----------------------------------------------------------0000

// func (at *ArticleTag) createArticleTag(ctx context.Context) error {
// 	logger := log.Ctx(ctx)
// 	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

// 	var insert = "INSERT INTO article_tag SET article_id=?, tag=?"

// 	_, err := db.Exec(insert, at.ArticleId, at.Tag)
// 	if err != nil {
// 		logger.Fatal().Err(err).
// 			Str("table_name", "article_tag").
// 			Msg("Failed on INSERT")
// 	}
// 	return err
// }
