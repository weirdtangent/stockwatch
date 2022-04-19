package main

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type TickerAttribute struct {
	TickerAttributeId int64  `db:"attribute_id"`
	TickerId          int64  `db:"ticker_id"`
	AttributeName     string `db:"attribute_name"`
	AttributeValue    string `db:"attribute_value"`
	CreateDatetime    string `db:"create_datetime"`
	UpdateDatetime    string `db:"update_datetime"`
}

func (ta *TickerAttribute) getByUniqueKey(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM ticker_attribute WHERE ticker_id=? AND attribute_name=?`, ta.TickerId, ta.AttributeName).StructScan(ta)
	return err
}
