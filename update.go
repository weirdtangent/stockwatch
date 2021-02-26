package main

import (
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/mytime"
)

func updateHandler(aws *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		action := params["action"]

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
			symbol := params["symbol"]
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
func updateTickerWithEOD(aws *session.Session, db *sqlx.DB, ticker *Ticker) (*Ticker, error) {
	mostRecentDaily, err := getDailyMostRecent(db, ticker.Ticker_id)
	if err != nil {
		log.Warn().Err(err).
			Str("symbol", ticker.Ticker_symbol).
			Int64("ticker_id", ticker.Ticker_id).
			Msg("Error getting most recent EOD date")
		return ticker, err
	}
	mostRecentDailyDate := mostRecentDaily.Price_date
	mostRecentAvailable := mostRecentPricesAvailable()

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

func mostRecentPricesAvailable() string {
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

	prevWorkDate := mytime.PriorWorkDate(currentDateTime)
	prevWorkDay := prevWorkDate.Format("2006-01-02")

	return prevWorkDay
}

// see if we need to pull intradays for the selected date:
//  if we don't have the intraday prices for the selected date
//  AND it was a prior business day or today and it's now 7pm or later
func updateTickerWithIntraday(aws *session.Session, db *sqlx.DB, ticker *Ticker, intradate string) (bool, error) {
	haveIntradayData, err := gotIntradayData(db, ticker.Ticker_id, intradate)
	if err != nil {
		log.Warn().Err(err).
			Str("symbol", ticker.Ticker_symbol).
			Int64("ticker_id", ticker.Ticker_id).
			Str("intraday", intradate).
			Msg("Error getting intradate data")
		return false, err
	}
	if haveIntradayData {
		return haveIntradayData, nil
	}

	exchange, err := getExchangeById(db, ticker.Exchange_id)
	if err != nil {
		log.Fatal().Err(err).
			Str("symbol", ticker.Ticker_symbol).
			Int64("ticker_id", ticker.Ticker_id).
			Int64("exchange_td", ticker.Exchange_id).
			Msg("Failed to find exchange")
	}

	mostRecentAvailable := mostRecentPricesAvailable()

	log.Info().
		Str("symbol", ticker.Ticker_symbol).
		Str("acronym", exchange.Exchange_acronym).
		Str("intraday", intradate).
		Str("mostRecentAvailable", mostRecentAvailable).
		Msg("check if intraday data available for ticker")

	if intradate <= mostRecentAvailable {
		err = updateMarketstackIntraday(aws, db, ticker, exchange, intradate)
		if err != nil {
			log.Warn().Err(err).
				Str("symbol", ticker.Ticker_symbol).
				Int64("ticker_id", ticker.Ticker_id).
				Str("intraday", intradate).
				Msg("Error getting intraday prices for ticker")
			return false, err
		}
		log.Info().
			Str("symbol", ticker.Ticker_symbol).
			Int64("ticker_id", ticker.Ticker_id).
			Str("intraday", intradate).
			Msg("Updated ticker with intraday prices")
	}

	return true, nil
}
