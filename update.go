package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	//"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/mytime"
)

func updateHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		messages := ctx.Value("messages").(*[]Message)

		if ok := checkAuthState(w, r); ok == false {
			http.Redirect(w, r, "/", 307)
		} else {
			params := mux.Vars(r)
			action := params["action"]

			switch action {
			case "movers":
				err := loadMovers(ctx)
				if err != nil {
					*messages = append(*messages, Message{fmt.Sprintf("Pulling latest Morningstar Movers failed: %s", err.Error()), "danger"})
				} else {
					*messages = append(*messages, Message{fmt.Sprintf("Pulled latest Morningstar Movers"), "success"})
				}
			default:
				logger.Error().Str("action", action).Msg("Unknown update action")
				*messages = append(*messages, Message{fmt.Sprintf("Unknown update action: %s", action), "danger"})
			}

			logger.Info().Msgf("Update operation ended normally")
			renderTemplateDefault(w, r, "update")
		}
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

	if IsWorkDay && currentTime > "16:00:00" {
		return currentDate
	}

	prevWorkDate := mytime.PriorWorkDate(currentDateTime)
	prevWorkDay := prevWorkDate.Format("2006-01-02")

	return prevWorkDay
}
