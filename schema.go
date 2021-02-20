package stockwatch

import (
	"database/sql"
	"html/template"
)

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
	Exchange_name    string
	Country_id       int64
	City             string
	Create_datetime  string
	Update_datetime  string
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

// contrived schema for views

type NilView struct {
}

type WebWatch struct {
	Source_date    string
	Target_price   float32
	Target_date    sql.NullString
	Source_name    sql.NullString
	Source_company sql.NullString
}

type TickerView struct {
	Ticker         Ticker
	Exchange       Exchange
	Dailies        []Daily
	Watches        []WebWatch
	LineChartHTML  template.HTML
	KLineChartHTML template.HTML
}

type MessageView struct {
	Messages []string
}

// marketstack data

type MSExchangeData struct {
	Name         string `json:"name"`
	Acronym      string `json:"acronym"`
	Country      string `json:"country"`
	Country_code string `json:"country_code"`
	City         string `json:"city"`
}

type MSIndexData struct {
	Symbol      string  `json:"symbol"`
	Exchange    string  `json:"exchange"`
	Price_date  string  `json:"date"`
	Volume      float32 `json:"volume"`
	Open_price  float32 `json:"open"`
	High_price  float32 `json:"high"`
	Low_price   float32 `json:"low"`
	Close_price float32 `json:"close"`
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

type MSTickerResponse struct {
	Data []MSTickerData `json:"data"`
}
