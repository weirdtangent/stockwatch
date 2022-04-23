package main

import "database/sql"

type Currency struct {
	CurrencyId           uint64       `db:"currency_id"`
	CurrencyCode         string       `db:"currency_code"`
	CurrencyName         string       `db:"currency_name"`
	CurrencySymbol       string       `db:"currency_symbol"`
	CurrencySymbolNative string       `db:"currency_symbol_native"`
	CreateDatetime       sql.NullTime `db:"create_datetime"`
	UpdateDatetime       sql.NullTime `db:"update_datetime"`
}
