package main

import (
	"github.com/rs/zerolog"

	"github.com/weirdtangent/myaws"
)

const (
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
	// connect to AWS
	awssess, err := myaws.AWSConnect("us-east-1", "stockwatch")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to connect to AWS")
	}

	// connect to MySQL
	db := myaws.DBMustConnect(awssess, "stockwatch")

	_, err = db.Exec("SET NAMES utf8mb4 COLLATE utf8mb4_general_ci")
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("failed to switch RDS to UTF8")
	}

	// Setup secrets, sessions, oauth, and start HTTP server
	secrets := getSecrets(ctx, awssess)
	secureCookie, store := setupSessionsStore(ctx, awssess)
	setupOAuth(ctx, awssess, store)

	startHTTPServer(ctx, awssess, db, secrets, store, secureCookie)
}
