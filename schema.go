package main

import (
	"database/sql"
	"time"
)

// table schema from aurora ---------------------------------------------------

type Country struct {
	CountryId      uint64    `db:"country_id"`
	CountryCode    string    `db:"country_code"`
	CountryName    string    `db:"country_name"`
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}

type Source struct {
	SourceId       uint64    `db:"source_id"`
	SourceCompany  string    `db:"source_company"`
	SourceName     string    `db:"source_name"`
	SourceWebsite  string    `db:"source_website"`
	SourceEmail    string    `db:"source_email"`
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}

type Watch struct {
	WatchId        uint64       `db:"watch_id"`
	TickerId       uint64       `db:"ticker_id"`
	SourceId       uint64       `db:"source_id"`
	SourceDate     string       `db:"source_date"`
	TargetPrice    float64      `db:"target_price"`
	TargetDate     sql.NullTime `db:"target_date"`
	CreateDatetime time.Time    `db:"create_datetime"`
	UpdateDatetime time.Time    `db:"update_datetime"`
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
	ViewQuote     struct {
		QuoteRefresh int
	}
}

type WebWatch struct {
	SourceDate    string
	TargetPrice   float64
	TargetDate    sql.NullString
	SourceName    sql.NullString
	SourceCompany sql.NullString
}

type Message struct {
	Text  string
	Level string
}

type Messages struct {
	Messages []Message
}
