package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"graystorm.com/mytime"
)

func updateHandler(aws *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path_paramlist := r.URL.Path[len("/update/"):]
		params := strings.Split(path_paramlist, "/")
		action := params[0]

		switch action {
		case "exchanges":
			success, err := updateMarketstackExchanges(aws, db)
			if err != nil {
				log.Error().Msgf("Bulk update of Exchanges failed: %s", err)
				return
			}
			if success != true {
				log.Error().Msgf("Bulk update of Exchanges failed")
				return
			}
		case "ticker":
			symbol := params[1]
			_, err := updateMarketstackTicker(aws, db, symbol)
			if err != nil {
				log.Error().Msgf("Update of ticket symbol %s failed: %s", symbol, err)
				return
			}
		default:
			log.Error().
				Str("action", action).
				Msg("Unknown update action")
		}

		log.Info().Msgf("Operation completed normally")
	})
}

// see if we need to pull a daily update:
//  if we don't have the EOD price for the prior business day
//  OR if we don't have it for the current business day and it's now 7pm or later
func updateTicker(aws *session.Session, db *sqlx.DB, ticker *Ticker) (*Ticker, error) {
	mostRecentDaily, err := getDailyMostRecent(db, ticker.Ticker_id)
	if err != nil {
		log.Warn().Err(err).
			Str("symbol", ticker.Ticker_symbol).
			Int64("ticker_id", ticker.Ticker_id).
			Msg("Error getting most recent EOD date")
		return ticker, err
	}
	mostRecentDailyDate := mostRecentDaily.Price_date
	mostRecentAvailable := mostRecentEODPricesAvailable()

	log.Info().
		Str("symbol", ticker.Ticker_symbol).
		Str("mostRecentDailyDate", mostRecentDailyDate).
		Str("mostRecentAvailable", mostRecentAvailable).
		Msg("check if new EOD available for ticker")

	if mostRecentDailyDate < mostRecentAvailable {
		ticker, err = updateMarketstackTicker(aws, db, ticker.Ticker_symbol)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.Ticker_symbol).
				Int64("ticker_id", ticker.Ticker_id).
				Msg("Error getting EOD prices for ticker")
			return ticker, err
		}
		log.Info().
			Str("symbol", ticker.Ticker_symbol).
			Int64("ticker_id", ticker.Ticker_id).
			Msg("Updated ticker with latest EOD prices")
	}

	return ticker, nil
}

func mostRecentEODPricesAvailable() string {
	EasternTZ, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Error().Err(err).
			Msg("Failed to get timezone")
		return "1970-01-01"
	}
	currentDateTime := time.Now().In(EasternTZ)
	currentTime := currentDateTime.Format("15:04:05")
	currentDate := currentDateTime.Format("2006-01-02")
	IsWorkDay := mytime.IsWorkday(currentDateTime)

	if IsWorkDay && currentTime > "19:00:00" {
		return currentDate
	}

	prevWorkDate := mytime.LastWorkDate(currentDateTime)
	prevWorkDay := prevWorkDate.Format("2006-01-02")

	return prevWorkDay
}
