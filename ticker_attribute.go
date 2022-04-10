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

// func (ta *TickerAttribute) createIfNew(ctx context.Context) error {
// 	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
// 	logger := log.Ctx(ctx)

// 	if ta.AttributeName == "" || ta.AttributeValue == "" {
// 		logger.Warn().Msg("Refusing to add ticker attribute with blank attribute name or value")
// 		return nil
// 	}

// 	err := ta.getByUniqueKey(ctx)
// 	if err == nil {
// 		return nil
// 	}

// 	var insert = "INSERT INTO ticker_attribute SET ticker_id=?, attribute_name=?, attribute_value=?"
// 	_, err = db.Exec(insert, ta.TickerId, ta.AttributeName, ta.AttributeValue)
// 	if err != nil {
// 		logger.Fatal().Err(err).
// 			Str("table_name", "ticker_attribute").
// 			Msg("Failed on INSERT")
// 	}
// 	return err
// }
