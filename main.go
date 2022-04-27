package main

import (
	"github.com/weirdtangent/myaws"
)

const (
	httpPort = 3001

	skipRedisChecks     = false // always skip the redis cache info
	skipLocalTickerInfo = false // always fetch ticker info from yhfinance

	sqlDateParseType      = "2006-01-02"
	sqlDatetimeParseType  = "2006-01-02T15:04:05Z"
	sqlDatetimeSearchType = "2006-01-02 15:04:05"

	volumeUnits = 1_000_000 // factor to reduce volume counts by when graphing

	debugging = true // output DEBUG level logs
)

func main() {
	ctx := setupLogging()

	awssess := myaws.AWSMustConnect("us-east-1", "stockwatch")
	db := myaws.DBMustConnect(awssess, "stockwatch")

	secrets := getSecrets(ctx, awssess)
	secureCookie, store := setupSessionsStore(ctx, awssess)
	setupOAuth(ctx, awssess, store)

	startHTTPServer(ctx, awssess, db, secrets, store, secureCookie)
}
