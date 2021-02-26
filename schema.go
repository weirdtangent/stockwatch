package main

import (
	"database/sql"
	"html/template"
)

// table schema from aurora ---------------------------------------------------

type Country struct {
	Country_id      int64
	Country_code    string
	Country_name    string
	Create_datetime string
	Update_datetime string
}

type Daily struct {
	Daily_id        int64
	Ticker_id       int64
	Price_date      string
	Open_price      float32
	High_price      float32
	Low_price       float32
	Close_price     float32
	Volume          float32
	Create_datetime string
	Update_datetime string
}

type Exchange struct {
	Exchange_id      int64
	Exchange_acronym string
	Exchange_mic     string
	Exchange_name    string
	Country_id       int64
	City             string
	Create_datetime  string
	Update_datetime  string
}

type Intraday struct {
	Intraday_id     int64
	Ticker_id       int64
	Price_date      string
	Last_price      float32
	Volume          float32
	Create_datetime string
	Update_datetime string
}

type OAuth struct {
	OAuth_id          int64
	OAuth_vener       string
	OAuth_user        string
	Watcher_id        int64
	OAuth_status      string
	LastUser_datetime string
	Create_datetime   string
	Update_datetime   string
}

type Source struct {
	Source_id       int64
	Source_company  string
	Source_name     string
	Source_website  string
	Source_email    string
	Create_datetime string
	Update_datetime string
}

type Ticker struct {
	Ticker_id       int64
	Ticker_symbol   string
	Exchange_id     int64
	Ticker_name     string
	Create_datetime string
	Update_datetime string
}

type Watch struct {
	Watch_id        int64
	Ticker_id       int64
	Source_id       int64
	Source_date     string
	Target_price    float32
	Target_date     sql.NullString
	Create_datetime string
	Update_datetime string
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
	Source_date    string
	Target_price   float32
	Target_date    sql.NullString
	Source_name    sql.NullString
	Source_company sql.NullString
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
	Name         string `json:"name"`
	Acronym      string `json:"acronym"`
	Mic          string `json:"mic"`
	Country      string `json:"country"`
	Country_code string `json:"country_code"`
	City         string `json:"city"`
}

type MSIndexData struct {
	Symbol      string  `json:"symbol"`
	Exchange    string  `json:"exchange"`
	Price_date  string  `json:"date"`
	Open_price  float32 `json:"open"`
	High_price  float32 `json:"high"`
	Low_price   float32 `json:"low"`
	Close_price float32 `json:"close"`
	Volume      float32 `json:"volume"`
}

type MSIntradayData struct {
	Symbol     string  `json:"symbol"`
	Exchange   string  `json:"exchange"`
	Price_date string  `json:"date"`
	Last_price float32 `json:"last"`
	Volume     float32 `json:"volume"`
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
