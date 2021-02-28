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

func updateHandler(awssess *session.Session, db *sqlx.DB) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		action := params["action"]

		switch action {
		case "exchanges":
			success, err := updateMarketstackExchanges(awssess, db)
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
			_, err := updateMarketstackTicker(awssess, db, symbol)
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
