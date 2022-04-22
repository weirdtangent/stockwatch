package main

type Financials struct {
	FinancialsId    int64   `db:"financials_id"`
	TickerId        int64   `db:"ticker_id"`
	FormName        string  `db:"form_name"`
	FormTermName    string  `db:"form_term_name"`
	ChartName       string  `db:"chart_name"`
	ChartDateString string  `db:"chart_date_string"`
	ChartType       string  `db:"chart_type"`
	IsPercentage    bool    `db:"is_percentage"`
	ChartValue      float64 `db:"chart_value"`
	CreateDatetime  string  `db:"create_datetime"`
	UpdateDatetime  string  `db:"update_datetime"`
}

type BarFinancials struct {
	Values map[string]float64
}

type QuarterlyFinancials struct {
	Quarters []BarFinancials
}