package main

type LastDone struct {
	Activity         string `db:"activity"`
	UniqueKey        string `db:"unique_key"`
	LastStatus       string `db:"last_status"`
	LastDoneDatetime string `db:"lastdone_datetime"`
	CreateDatetime   string `db:"create_datetime"`
	UpdateDatetime   string `db:"update_datetime"`
}

// object methods -------------------------------------------------------------

// misc -----------------------------------------------------------------------