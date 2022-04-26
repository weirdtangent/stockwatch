package main

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type LastDone struct {
	Activity         string       `db:"activity"`
	UniqueKey        string       `db:"unique_key"`
	LastStatus       string       `db:"last_status"`
	LastDoneDatetime sql.NullTime `db:"lastdone_datetime"`
	CreateDatetime   sql.NullTime `db:"create_datetime"`
	UpdateDatetime   sql.NullTime `db:"update_datetime"`
}

const (
	minTickerNewsDelay       = 60 * 4
	minTickerFinancialsDelay = 60 * 24
)

// object methods -------------------------------------------------------------
func (ld *LastDone) getByActivity(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM lastdone WHERE activity=? AND unique_key=?", ld.Activity, ld.UniqueKey).StructScan(ld)
	return err
}

// misc -----------------------------------------------------------------------
