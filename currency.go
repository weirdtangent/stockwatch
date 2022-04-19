package main

type Currency struct {
	CurrencyId           int64  `db:"currency_id"`
	CurrencyCode         string `db:"currency_code"`
	CurrencyName         string `db:"currency_name"`
	CurrencySymbol       string `db:"currency_symbol"`
	CurrencySymbolNative string `db:"currency_symbol_native"`
	CreateDatetime       string `db:"create_datetime"`
	UpdateDatetime       string `db:"update_datetime"`
}
