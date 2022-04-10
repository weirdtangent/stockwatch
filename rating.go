package main

type Rating struct {
	RatingId       int64  `db:"rating_id"`
	RatingType     string `db:"rating_type"`
	TypeId         int64  `db:"type_id"`
	RaterId        int64  `db:"rater_id"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

// func getRatingById(db *sqlx.DB, ratingId int64) (*Rating, error) {
// 	var rating Rating
// 	err := db.QueryRowx("SELECT * FROM rating WHERE rating_id=?", ratingId).StructScan(&rating)
// 	return &rating, err
// }

// func createRating(db *sqlx.DB, rating *Rating) (*Rating, error) {
// 	var insert = "INSERT INTO rating SET rating_type=?, type_id=?, rater_id=?"

// 	res, err := db.Exec(insert, rating.RatingType, rating.TypeId, rating.RaterId)
// 	if err != nil {
// 		log.Fatal().Err(err).
// 			Str("table_name", "rating").
// 			Msg("Failed on INSERT")
// 	}
// 	ratingId, err := res.LastInsertId()
// 	if err != nil {
// 		log.Fatal().Err(err).
// 			Str("table_name", "rating").
// 			Msg("Failed on LAST_INSERT_ID")
// 	}
// 	return getRatingById(db, ratingId)
// }
