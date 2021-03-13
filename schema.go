package main

import (
	"database/sql"
)

// table schema from aurora ---------------------------------------------------

type Country struct {
	CountryId      int64  `db:"country_id"`
	CountryCode    string `db:"country_code"`
	CountryName    string `db:"country_name"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

type Source struct {
	SourceId       int64  `db:"source_id"`
	SourceCompany  string `db:"source_company"`
	SourceName     string `db:"source_name"`
	SourceWebsite  string `db:"source_website"`
	SourceEmail    string `db:"source_email"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

type Watch struct {
	WatchId        int64          `db:"watch_id"`
	TickerId       int64          `db:"ticker_id"`
	SourceId       int64          `db:"source_id"`
	SourceDate     string         `db:"source_date"`
	TargetPrice    float32        `db:"target_price"`
	TargetDate     sql.NullString `db:"target_date"`
	CreateDatetime string         `db:"create_datetime"`
	UpdateDatetime string         `db:"update_datetime"`
}

// google oauth ---------------------------------------------------------------

type GoogleProfileData struct {
	Name       string
	GivenName  string
	FamilyName string
	Email      string
	PictureURL string
	Locale     string
}

// contrived schema for templates ---------------------------------------------

type ConfigData struct {
	TmplName      string
	GoogleProfile GoogleProfileData
}

type WebWatch struct {
	SourceDate    string
	TargetPrice   float32
	TargetDate    sql.NullString
	SourceName    sql.NullString
	SourceCompany sql.NullString
}

type ViewPair struct {
	Symbol  string
	Acronym string
}

type Message struct {
	Text  string
	Level string
}

type Messages struct {
	Messages []Message
}
