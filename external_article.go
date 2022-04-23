package main

import "database/sql"

type ExternalArticle struct {
	ExternalArticleId uint64       `db:"external_article_id"`
	SubmitterId       uint64       `db:"submitter_id"`
	LinkTitle         string       `db:"link_title"`
	LinkDesc          string       `db:"link_desc"`
	LinkURL           string       `db:"link_url"`
	WatchId           uint64       `db:"watch_id"`
	CreateDatetime    sql.NullTime `db:"create_datetime"`
	UpdateDatetime    sql.NullTime `db:"update_datetime"`
}
