package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"html/template"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"
)

type Article struct {
	ArticleId          uint64       `db:"article_id"`
	SourceId           uint64       `db:"source_id"`
	ExternalId         string       `db:"external_id"`
	PublishedDatetime  sql.NullTime `db:"published_datetime"`
	PubUpdatedDatetime sql.NullTime `db:"pubupdated_datetime"`
	Title              string       `db:"title"`
	Body               string       `db:"body"`
	ArticleURL         string       `db:"article_url"`
	ImageURL           string       `db:"image_url"`
	CreateDatetime     sql.NullTime `db:"create_datetime"`
	UpdateDatetime     sql.NullTime `db:"update_datetime"`
}

type ArticleTicker struct {
	ArticleTickerId uint64       `db:"article_ticker_id"`
	ArticleId       uint64       `db:"article_id"`
	TickerSymbol    string       `db:"ticker_symbol"`
	TickerId        uint64       `db:"ticker_id"`
	CreateDatetime  sql.NullTime `db:"create_datetime"`
	UpdateDatetime  sql.NullTime `db:"update_datetime"`
}

type ArticleKeyword struct {
	ArticleKeywordId uint64       `db:"article_keyword_id"`
	ArticleId        uint64       `db:"article_id"`
	Keyword          string       `db:"keyword"`
	CreateDatetime   sql.NullTime `db:"create_datetime"`
	UpdateDatetime   sql.NullTime `db:"update_datetime"`
}

type ArticleAuthor struct {
	ArticleAuthorId uint64       `db:"article_author_id"`
	ArticleId       uint64       `db:"article_id"`
	Byline          string       `db:"byline"`
	JobTitle        string       `db:"job_title"`
	ShortBio        string       `db:"short_bio"`
	LongBio         string       `db:"long_bio"`
	ImageURL        string       `db:"image_url"`
	CreateDatetime  sql.NullTime `db:"create_datetime"`
	UpdateDatetime  sql.NullTime `db:"update_datetime"`
}

type ArticleTag struct {
	ArticleTagId   uint64       `db:"article_tag_id"`
	ArticleId      uint64       `db:"article_id"`
	Tag            string       `db:"tag"`
	CreateDatetime sql.NullTime `db:"create_datetime"`
	UpdateDatetime sql.NullTime `db:"update_datetime"`
}

type WebArticle struct {
	ArticleId          uint64       `db:"article_id"`
	SourceId           uint64       `db:"source_id"`
	ExternalId         string       `db:"external_id"`
	PublishedDatetime  sql.NullTime `db:"published_datetime"`
	PubUpdatedDatetime sql.NullTime `db:"pubupdated_datetime"`
	Title              string       `db:"title"`
	Body               string       `db:"body"`
	BodyTemplate       template.HTML
	ArticleURL         string         `db:"article_url"`
	ImageURL           string         `db:"image_url"`
	CreateDatetime     sql.NullTime   `db:"create_datetime"`
	UpdateDatetime     sql.NullTime   `db:"update_datetime"`
	AuthorByline       sql.NullString `db:"author_byline"`
	AuthorLongBio      sql.NullString `db:"author_long_bio"`
	AuthorImageURL     sql.NullString `db:"author_image_url"`
	SourceName         sql.NullString `db:"source_name"`
	Keywords           sql.NullString `db:"keywords"`
	Tags               sql.NullString `db:"tags"`
	Symbols            sql.NullString `db:"symbols"`
}

// func getArticlesByKeyword(deps *Dependencies, keyword string) ([]WebArticle, error) {
// 	db := deps.db

// 	if keyword == "" {
// 		return []WebArticle{}, fmt.Errorf("missing required keyword param")
// 	}

// 	fromDate := time.Now().AddDate(0, 0, -120).Format(sqlDatetimeSearchType)
// 	query := `SELECT article.article_id, article.source_id, article.external_id, article.published_datetime, article.pubupdated_datetime,
// 		            article.title, article.body, article.article_url, article.image_url,
//                     ANY_VALUE(article_author.byline) AS author_byline,
// 					ANY_VALUE(article_author.long_bio) AS author_long_bio,
// 					ANY_VALUE(article_author.image_url) AS author_image_url,
// 					source.source_name AS source_name,
// 					GROUP_CONCAT(DISTINCT article_keyword.keyword ORDER BY article_keyword.keyword SEPARATOR ', ') AS keywords,
// 					GROUP_CONCAT(DISTINCT article_tag.tag ORDER BY article_tag.tag SEPARATOR ', ') AS tags,
// 					GROUP_CONCAT(DISTINCT article_ticker.ticker_symbol ORDER BY article_ticker.ticker_symbol SEPARATOR ', ') AS symbols
// 				FROM article
//  				LEFT JOIN article_author USING (article_id)
// 				LEFT JOIN article_ticker USING (article_id)
// 				LEFT JOIN article_keyword USING (article_id)
// 				LEFT JOIN article_tag USING (article_id)
// 				LEFT JOIN source USING (source_id)
// 				WHERE published_datetime > ?
// 					AND (keyword=? OR tag=? OR ticker_symbol=?)
//   				GROUP BY article_id
// 				ORDER BY published_datetime DESC
//     			LIMIT 6`
// 	rows, err := db.Queryx(query, fromDate, keyword, keyword, keyword)
// 	if err != nil {
// 		log.Warn().Err(err).Msg("Failed to check for articles")
// 		return []WebArticle{}, err
// 	}
// 	defer rows.Close()

// 	bodySHA256 := make(map[string]bool)

// 	var article WebArticle
// 	articles := make([]WebArticle, 0)
// 	for rows.Next() {
// 		err = rows.StructScan(&article)
// 		if err != nil {
// 			log.Warn().Err(err).Str("table_name", "article,article_author").Msg("Error reading result rows")
// 			continue
// 		}
// 		if len(article.Body) > 0 {
// 			sha := fmt.Sprintf("%x", sha256.Sum256([]byte(article.Body)))
// 			if _, ok := bodySHA256[sha]; ok {
// 				sublog.Info().Msg("Skipping, seen this article body already")
// 			} else {
// 				bodySHA256[sha] = true

// 				quote_rx := regexp.MustCompile(`'`)
// 				article.Body = string(quote_rx.ReplaceAll([]byte(article.Body), []byte("&apos;")))

// 				http_rx := regexp.MustCompile(`http:`)
// 				article.Body = string(http_rx.ReplaceAll([]byte(article.Body), []byte("https:")))
// 				article.AuthorImageURL.String = string(http_rx.ReplaceAll([]byte(article.AuthorImageURL.String), []byte("https:")))

// 				articles = append(articles, article)
// 			}
// 		} else {
// 			articles = append(articles, article)
// 		}
// 	}
// 	if err := rows.Err(); err != nil {
// 		return []WebArticle{}, err
// 	}

// 	return articles, nil
// }

func getArticlesByTicker(deps *Dependencies, ticker_id uint64) ([]WebArticle, error) {
	db := deps.db

	// go back as far as 180 days but limited to 20 articles
	fromDate := time.Now().Add(-1 * 180 * 24 * time.Hour).Format(sqlDatetimeSearchType)
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
			  WHERE published_datetime > ?
			     AND ticker_id=?
  			  GROUP BY article_id
			  ORDER BY published_datetime DESC
    		  LIMIT 20`
	rows, err := db.Queryx(query, fromDate, ticker_id)

	if err != nil {
		log.Warn().Err(err).Msg("Failed to check for articles")
		return []WebArticle{}, err
	}
	defer rows.Close()

	bodySHA256 := make(map[string]bool)

	var article WebArticle
	articles := make([]WebArticle, 0)
	for rows.Next() {
		err = rows.StructScan(&article)
		if err != nil {
			log.Warn().Err(err).Str("table_name", "article,article_author").Msg("Error reading result rows")
			continue
		}
		sha := fmt.Sprintf("%x", sha256.Sum256([]byte(article.Title)))
		// skip this one if we've seen the same title already
		if _, ok := bodySHA256[sha]; ok {
			continue
		}
		bodySHA256[sha] = true

		quote_rx := regexp.MustCompile(`'`)
		article.Body = string(quote_rx.ReplaceAll([]byte(article.Body), []byte("&apos;")))

		http_rx := regexp.MustCompile(`http:`)
		article.Body = string(http_rx.ReplaceAll([]byte(article.Body), []byte("https:")))
		article.AuthorImageURL.String = string(http_rx.ReplaceAll([]byte(article.AuthorImageURL.String), []byte("https:")))

		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return []WebArticle{}, err
	}

	return articles, nil
}

func getRecentArticles(deps *Dependencies) ([]WebArticle, error) {
	db := deps.db
	sublog := deps.logger

	// go back as far as 30 days but limited to 30 articles
	fromDate := time.Now().Add(-1 * 30 * 24 * time.Hour).Format(sqlDatetimeSearchType)
	sublog.Info().Str("from_date", fromDate).Msg("checking for recent news since {from_date}")

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
		log.Warn().Err(err).Msg("Failed to check for articles")
		return []WebArticle{}, err
	}
	defer rows.Close()

	bodySHA256 := make(map[string]bool)

	var article WebArticle
	articles := make([]WebArticle, 0)
	for rows.Next() {
		err = rows.StructScan(&article)
		if err != nil {
			log.Warn().Err(err).Str("table_name", "article,article_author").Msg("Error reading result rows")
			continue
		}
		sha := fmt.Sprintf("%x", sha256.Sum256([]byte(article.Title)))
		// skip this one if we've seen the same title already
		if _, ok := bodySHA256[sha]; ok {
			continue
		}
		bodySHA256[sha] = true

		quote_rx := regexp.MustCompile(`'`)
		article.Body = quote_rx.ReplaceAllString(article.Body, "&apos;")

		http_rx := regexp.MustCompile(`http:`)
		article.Body = http_rx.ReplaceAllString(article.Body, "https:")
		article.AuthorImageURL.String = http_rx.ReplaceAllString(article.AuthorImageURL.String, "https:")

		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return []WebArticle{}, err
	}

	return articles, nil
}

func getNewsLastUpdated(deps *Dependencies, ticker Ticker) (sql.NullTime, bool) {
	webdata := deps.webdata
	sublog := deps.logger

	newsLastUpdated := sql.NullTime{Valid: false, Time: time.Time{}}
	updatingNewsNow := false
	lastdone := LastDone{Activity: "ticker_news", UniqueKey: ticker.TickerSymbol}
	err := lastdone.getByActivity(deps)
	if err != nil {
		sublog.Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to get LastDone activity for {symbol}")
		webdata["NewsLastUpdated"] = newsLastUpdated
		webdata["UpdatingNewsNow"] = updatingNewsNow
		return sql.NullTime{}, false
	}
	if lastdone.LastStatus == "success" {
		newsLastUpdated = sql.NullTime{Valid: true, Time: lastdone.LastDoneDatetime.Time}
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerNewsDelay).Before(time.Now()) {
			err = ticker.queueUpdateNews(deps)
			updatingNewsNow = (err == nil)
		} else {
			if webdata["TZLocation"] != nil {
				location, err := time.LoadLocation(webdata["TZLocation"].(string))
				if err != nil {
					location, _ = time.LoadLocation("UTC")
				}
				newsLastUpdated = sql.NullTime{Valid: true, Time: lastdone.LastDoneDatetime.Time.In(location)}
			} else {
				newsLastUpdated = sql.NullTime{Valid: true, Time: lastdone.LastDoneDatetime.Time}
			}
			updatingNewsNow = false
		}
	} else {
		sublog.Info().Str("symbol", ticker.TickerSymbol).Msg("last try failed, lets try to get news again")
		err = ticker.queueUpdateNews(deps)
		updatingNewsNow = (err == nil)
	}

	return newsLastUpdated, updatingNewsNow
}
