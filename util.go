package main

import (
	"time"
)

func RealDateForHuman(unixTime int64) string {
	EasternTZ, _ := time.LoadLocation("America/New_York")
	realDate := time.Unix(unixTime, 0).In(EasternTZ)
	return realDate.Format("Jan 2 15:04 MST 2006")
}

func RealDateForDB(unixTime int64) string {
	EasternTZ, _ := time.LoadLocation("America/New_York")
	realDate := time.Unix(unixTime, 0).In(EasternTZ)
	return realDate.Format("2006-01-02")
}
