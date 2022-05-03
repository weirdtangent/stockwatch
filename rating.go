package main

import "time"

type Rating struct {
	RatingId       uint64    `db:"rating_id"`
	RatingType     string    `db:"rating_type"`
	TypeId         uint64    `db:"type_id"`
	RaterId        uint64    `db:"rater_id"`
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}
