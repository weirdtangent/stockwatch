package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"html/template"
	"regexp"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Article struct {
	ArticleId          uint64 `db:"article_id"`
	EId                string
	SourceId           uint64       `db:"source_id"`
	ExternalId         string       `db:"external_id"`
	PublishedDatetime  sql.NullTime `db:"published_datetime"`
	PubUpdatedDatetime sql.NullTime `db:"pubupdated_datetime"`
	Title              string       `db:"title"`
	Body               string       `db:"body"`
	ArticleURL         string       `db:"article_url"`
	ImageURL           string       `db:"image_url"`
	CreateDatetime     time.Time    `db:"create_datetime"`
	UpdateDatetime     time.Time    `db:"update_datetime"`
}

type ArticleTicker struct {
	ArticleTickerId uint64 `db:"article_ticker_id"`
	EId             string
	ArticleId       uint64    `db:"article_id"`
	TickerSymbol    string    `db:"ticker_symbol"`
	TickerId        uint64    `db:"ticker_id"`
	CreateDatetime  time.Time `db:"create_datetime"`
	UpdateDatetime  time.Time `db:"update_datetime"`
}

type ArticleKeyword struct {
	ArticleKeywordId uint64 `db:"article_keyword_id"`
	EId              string
	ArticleId        uint64    `db:"article_id"`
	Keyword          string    `db:"keyword"`
	CreateDatetime   time.Time `db:"create_datetime"`
	UpdateDatetime   time.Time `db:"update_datetime"`
}

type ArticleAuthor struct {
	ArticleAuthorId uint64 `db:"article_author_id"`
	EId             string
	ArticleId       uint64    `db:"article_id"`
	Byline          string    `db:"byline"`
	JobTitle        string    `db:"job_title"`
	ShortBio        string    `db:"short_bio"`
	LongBio         string    `db:"long_bio"`
	ImageURL        string    `db:"image_url"`
	CreateDatetime  time.Time `db:"create_datetime"`
	UpdateDatetime  time.Time `db:"update_datetime"`
}

type ArticleTag struct {
	ArticleTagId   uint64 `db:"article_tag_id"`
	EId            string
	ArticleId      uint64    `db:"article_id"`
	Tag            string    `db:"tag"`
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}

type WebArticle struct {
	ArticleId          uint64 `db:"article_id"`
	EId                string
	SourceId           uint64       `db:"source_id"`
	ExternalId         string       `db:"external_id"`
	PublishedDatetime  sql.NullTime `db:"published_datetime"`
	PubUpdatedDatetime sql.NullTime `db:"pubupdated_datetime"`
	Title              string       `db:"title"`
	Body               string       `db:"body"`
	ArticleURL         string       `db:"article_url"`
	ExternalURL        bool
	ImageURL           string         `db:"image_url"`
	CreateDatetime     time.Time      `db:"create_datetime"`
	UpdateDatetime     time.Time      `db:"update_datetime"`
	AuthorByline       sql.NullString `db:"author_byline"`
	AuthorLongBio      sql.NullString `db:"author_long_bio"`
	AuthorImageURL     sql.NullString `db:"author_image_url"`
	SourceName         sql.NullString `db:"source_name"`
	Keywords           sql.NullString `db:"keywords"`
	Tags               sql.NullString `db:"tags"`
	Symbols            sql.NullString `db:"symbols"`
	BodyTemplate       template.HTML
}

// misc -----------------------------------------------------------------------

func getArticlesByTicker(deps *Dependencies, sublog zerolog.Logger, ticker Ticker, max int, goBack time.Duration) ([]WebArticle, error) {
	db := deps.db

	if max < 1 || max > 20 {
		max = 20
	}

	// to go back in time, our duration needs to be negative
	if goBack > 0 {
		goBack *= -1
	}

	fromDate := time.Now().Add(goBack).Format(sqlDatetimeSearchType)
	query := `SELECT article.article_id, article.source_id, article.external_id, article.published_datetime, article.pubupdated_datetime,
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
			  WHERE published_datetime > ? AND ticker_id=?
  			  GROUP BY article_id
			  ORDER BY published_datetime DESC
    		  LIMIT ?`
	rows, err := db.Queryx(query, fromDate, ticker.TickerId, max)
	if err != nil {
		return []WebArticle{}, err
	}

	defer rows.Close()
	bodySHA256 := make(map[string]bool)
	var article WebArticle
	articles := make([]WebArticle, 0)
	for rows.Next() {
		err = rows.StructScan(&article)
		if err != nil {
			log.Warn().Err(err).Msg("error reading row")
			continue
		}
		article.EId = encryptId(deps, sublog, "article", article.ArticleId)
		sha := fmt.Sprintf("%x", sha256.Sum256([]byte(article.Title)))
		// skip this one if we've seen the same title already
		if _, ok := bodySHA256[sha]; ok {
			continue
		}
		bodySHA256[sha] = true
		article.Body = cleanArticleText(article.Body)
		if article.AuthorImageURL.Valid {
			article.AuthorImageURL.String = cleanArticleText(article.AuthorImageURL.String)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return []WebArticle{}, err
	}

	return articles, nil
}

func cleanArticleText(text string) string {
	if text == "" {
		return ""
	}

	quote_re := regexp.MustCompile(`'`)
	text = quote_re.ReplaceAllString(text, "&apos;")

	http_re := regexp.MustCompile(`http:`)
	text = http_re.ReplaceAllString(text, "https:")

	policy := bluemonday.UGCPolicy()
	text = policy.Sanitize(text)

	return text
}

func getRecentArticles(deps *Dependencies, sublog zerolog.Logger) []WebArticle {
	db := deps.db

	// go back as far as 30 days but limited to 30 articles
	fromDate := time.Now().Add(-1 * 30 * 24 * time.Hour).Format(sqlDatetimeSearchType)

	query := `SELECT article.article_id, article.source_id, article.external_id, article.published_datetime, article.pubupdated_datetime,
		        article.title, article.body, article.article_url, article.image_url,
              	article_author.byline AS author_byline,
				article_author.long_bio AS author_long_bio,
			    article_author.image_url AS author_image_url,
				source.source_name AS source_name
			  FROM article
			  LEFT JOIN article_author USING (article_id)
			  LEFT JOIN source USING (source_id)
			  WHERE published_datetime > ?
			  ORDER BY published_datetime DESC
			  LIMIT 50`
	rows, err := db.Queryx(query, fromDate)

	if err != nil {
		sublog.Warn().Err(err).Msg("failed to check for articles")
		return []WebArticle{}
	}
	defer rows.Close()

	bodySHA256 := make(map[string]bool)

	var article WebArticle
	articles := make([]WebArticle, 0)
	for rows.Next() {
		err = rows.StructScan(&article)
		if err != nil {
			sublog.Warn().Err(err).Msg("error reading row")
			continue
		}
		article.EId = encryptId(deps, sublog, "article", article.ArticleId)
		sha := fmt.Sprintf("%x", sha256.Sum256([]byte(article.Title)))
		// skip this one if we've seen the same title already
		if _, ok := bodySHA256[sha]; ok {
			continue
		}
		bodySHA256[sha] = true
		article.ArticleId = 0

		article.Body = cleanArticleText(article.Body)
		if article.AuthorImageURL.Valid {
			article.AuthorImageURL.String = cleanArticleText(article.AuthorImageURL.String)
		}

		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		sublog.Warn().Err(err).Msg("error reading rows")
		return []WebArticle{}
	}

	return articles
}
