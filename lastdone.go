package main

import (
	"database/sql"
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
	minTickerNewsDelay       = 60 * 4  //  4 hours
	minTickerFinancialsDelay = 60 * 24 // 24 hours
)

// object methods -------------------------------------------------------------
func (ld *LastDone) getByActivity(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM lastdone WHERE activity=? AND unique_key=?", ld.Activity, ld.UniqueKey).StructScan(ld)
	return err
}

// misc -----------------------------------------------------------------------
