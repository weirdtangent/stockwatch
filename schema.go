package main

import (
	"database/sql"
	"html/template"
)

// table schema from aurora ---------------------------------------------------

type Country struct {
	CountryId      int64  `db:"country_id"`
	CountryCode    string `db:"country_code"`
	CountryName    string `db:"country_name"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

type Daily struct {
	DailyId        int64   `db:"daily_id"`
	TickerId       int64   `db:"ticker_id"`
	PriceDate      string  `db:"price_date"`
	OpenPrice      float32 `db:"open_price"`
	HighPrice      float32 `db:"high_price"`
	LowPrice       float32 `db:"low_price"`
	ClosePrice     float32 `db:"close_price"`
	Volume         float32 `db:"volume"`
	CreateDatetime string  `db:"create_datetime"`
	UpdateDatetime string  `db:"update_datetime"`
}

type Exchange struct {
	ExchangeId      int64  `db:"exchange_id"`
	ExchangeAcronym string `db:"exchange_acronym"`
	ExchangeMic     string `db:"exchange_mic"`
	ExchangeName    string `db:"exchange_name"`
	CountryId       int64  `db:"country_id"`
	City            string `db:"city"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

type Intraday struct {
	IntradayId     int64   `db:"intraday_id"`
	TickerId       int64   `db:"ticker_id"`
	PriceDate      string  `db:"price_date"`
	LastPrice      float32 `db:"last_price"`
	Volume         float32 `db:"volume"`
	CreateDatetime string  `db:"create_datetime"`
	UpdateDatetime string  `db:"update_datetime"`
}

type OAuth struct {
	OAuthId          int64  `db:"oauth_id"`
	OAuthVender      string `db:"oauth_vender"`
	OAuthUser        string `db:"oauth_user"`
	WatcherId        int64  `db:"watcher_id"`
	OAuthStatus      string `db:"oauth_status"`
	LastUsedDatetime string `db:"lastused_datetime"`
	CreateDatetime   string `db:"create_datetime"`
	UpdateDatetime   string `db:"update_datetime"`
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

type Ticker struct {
	TickerId       int64  `db:"ticker_id"`
	TickerSymbol   string `db:"ticker_symbol"`
	ExchangeId     int64  `db:"exchange_id"`
	TickerName     string `db:"ticker_name"`
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

type DefaultView struct {
	Config  ConfigData
	Recents []ViewPair
}

type Message struct {
	Config      ConfigData
	MessageText string
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

type Dailies struct {
	Days []Daily
}

type TickerDailyView struct {
	Config         ConfigData
	Ticker         Ticker
	Exchange       Exchange
	Daily          Daily
	LastDailyMove  string
	Dailies        Dailies
	Watches        []WebWatch
	Recents        []ViewPair
	LineChartHTML  template.HTML
	KLineChartHTML template.HTML
}

type TickerIntradayView struct {
	Config            ConfigData
	Ticker            Ticker
	Exchange          Exchange
	Daily             Daily
	LastDailyMove     string
	Intradate         string
	PriorBusinessDate string
	NextBusinessDate  string
	Intradays         []Intraday
	Watches           []WebWatch
	Recents           []ViewPair
	LineChartHTML     template.HTML
}

type MessageView struct {
	Messages []string
}

// marketstack json data ------------------------------------------------------

type MSExchangeData struct {
	Name        string `json:"name"`
	Acronym     string `json:"acronym"`
	Mic         string `json:"mic"`
	CountryName string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
}

type MSIndexData struct {
	Symbol     string  `json:"symbol"`
	Exchange   string  `json:"exchange"`
	PriceDate  string  `json:"date"`
	OpenPrice  float32 `json:"open"`
	HighPrice  float32 `json:"high"`
	LowPrice   float32 `json:"low"`
	ClosePrice float32 `json:"close"`
	Volume     float32 `json:"volume"`
}

type MSIntradayData struct {
	Symbol    string  `json:"symbol"`
	Exchange  string  `json:"exchange"`
	PriceDate string  `json:"date"`
	LastPrice float32 `json:"last"`
	Volume    float32 `json:"volume"`
}

type MSEndOfDayData struct {
	Symbol        string         `json:"symbol"`
	Name          string         `json:"name"`
	StockExchange MSExchangeData `json:"stock_exchange"`
	EndOfDay      []MSIndexData  `json:"eod"`
}

type MSTickerData struct {
	Symbol        string         `json:"symbol"`
	Name          string         `json:"name"`
	StockExchange MSExchangeData `json:"stock_exchange"`
}

type MSEndOfDayResponse struct {
	Data MSEndOfDayData `json:"data"`
}

type MSExchangeResponse struct {
	Data []MSExchangeData `json:"data"`
}

type MSIndexResponse struct {
	Data []MSIndexData `json:"data"`
}

type MSIntradayResponse struct {
	Data []MSIntradayData `json:"data"`
}
type MSTickerResponse struct {
	Data []MSTickerData `json:"data"`
}
