package main

import "database/sql"

type Rating struct {
	RatingId       uint64       `db:"rating_id"`
	RatingType     string       `db:"rating_type"`
	TypeId         uint64       `db:"type_id"`
	RaterId        uint64       `db:"rater_id"`
	CreateDatetime sql.NullTime `db:"create_datetime"`
	UpdateDatetime sql.NullTime `db:"update_datetime"`
}
