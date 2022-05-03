package main

import (
	"database/sql"
	"time"
)

type Financials struct {
	FinancialsId   uint64       `db:"financials_id"`
	TickerId       uint64       `db:"ticker_id"`
	FormName       string       `db:"form_name"`
	FormTermName   string       `db:"form_term_name"`
	ChartName      string       `db:"chart_name"`
	ChartDatetime  sql.NullTime `db:"chart_datetime"`
	ChartType      string       `db:"chart_type"`
	IsPercentage   bool         `db:"is_percentage"`
	ChartValue     float64      `db:"chart_value"`
	CreateDatetime time.Time    `db:"create_datetime"`
	UpdateDatetime time.Time    `db:"update_datetime"`
}

type BarFinancials struct {
	Values map[string]float64
}

type QuarterlyFinancials struct {
	Quarters []BarFinancials
}
