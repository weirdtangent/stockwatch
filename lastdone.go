package main

import (
	"database/sql"
	"time"
)

type LastDone struct {
	Activity         string       `db:"activity"`
	UniqueKey        string       `db:"unique_key"`
	LastStatus       string       `db:"last_status"`
	LastDoneDatetime sql.NullTime `db:"lastdone_datetime"`
	CreateDatetime   time.Time    `db:"create_datetime"`
	UpdateDatetime   time.Time    `db:"update_datetime"`
}

// object methods -------------------------------------------------------------
func (ld *LastDone) getByActivity(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM lastdone WHERE activity=? AND unique_key=?", ld.Activity, ld.UniqueKey).StructScan(ld)
	return err
}

// misc -----------------------------------------------------------------------
