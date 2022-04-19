package main

type Rating struct {
	RatingId       int64  `db:"rating_id"`
	RatingType     string `db:"rating_type"`
	TypeId         int64  `db:"type_id"`
	RaterId        int64  `db:"rater_id"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}
