package main

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
