package main

import "time"

type Currency struct {
	CurrencyId           uint64 `db:"currency_id"`
	EId                  string
	CurrencyCode         string    `db:"currency_code"`
	CurrencyName         string    `db:"currency_name"`
	CurrencySymbol       string    `db:"currency_symbol"`
	CurrencySymbolNative string    `db:"currency_symbol_native"`
	CreateDatetime       time.Time `db:"create_datetime"`
	UpdateDatetime       time.Time `db:"update_datetime"`
}

// object methods -------------------------------------------------------------

// misc -----------------------------------------------------------------------
