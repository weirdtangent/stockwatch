package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/mytime"
)

func updateHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		awssess := ctx.Value("awssess").(*session.Session)
		db := ctx.Value("db").(*sqlx.DB)
		messages := ctx.Value("messages").(*[]Message)

		if ok := checkAuthState(w, r); ok == false {
			http.Redirect(w, r, "/", 307)
		} else {
			params := mux.Vars(r)
			action := params["action"]

			switch action {
			case "exchanges":
				count, err := fetchExchanges(awssess, db)
				if err != nil {
					logger.Error().Msgf("Bulk update of Exchanges failed: %s", err)
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Exchanges failed: %s", err.Error()), "danger"})
				} else if count == 0 {
					logger.Error().Msgf("Bulk update of Exchanges failed, no error msg but 0 exchanges retrieved")
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Exchanges failed: no error msg but 0 exchanges retrieved"), "danger"})
				} else {
					logger.Info().Int("count", count).Msg("Update of exchanges completed")
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Exchanges succeeded: %d exchanges updates", count), "success"})
				}
			case "indexes":
				count, err := fetchMarketIndexes(awssess, db)
				if err != nil {
					logger.Error().Msgf("Bulk update of Market Indexes failed: %s", err)
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Market Indexes failed: %s", err.Error()), "danger"})
				} else if count == 0 {
					logger.Error().Msgf("Bulk update of Market Indexes failed, no error msg but 0 market indexes retrieved")
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Market Indexes failed: no error msg but 0 market indexes retrieved"), "danger"})
				} else {
					logger.Info().Int("count", count).Msg("Update of market indexes completed")
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Market Indexes succeeded: %d market indexes updates", count), "success"})
				}
			case "currencies":
				count, err := fetchCurrencies(awssess, db)
				if err != nil {
					logger.Error().Msgf("Bulk update of Currencies failed: %s", err)
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Currencies failed: %s", err.Error()), "danger"})
				} else if count == 0 {
					logger.Error().Msgf("Bulk update of Currencies failed, no error msg but 0 currencies retrieved")
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Currencies failed: no error msg but 0 currencies retrieved"), "danger"})
				} else {
					logger.Info().Int("count", count).Msg("Update of currencies completed")
					*messages = append(*messages, Message{fmt.Sprintf("Bulk update of Currencies succeeded: %d currencies updates", count), "success"})
				}
			case "ticker":
				symbol := params["symbol"]
				acronym := params["acronym"]
				exchange, err := getExchange(db, acronym)
				if err != nil {
					logger.Error().Msgf("Update of ticker symbol %s failed: %s", symbol, err)
					*messages = append(*messages, Message{fmt.Sprintf("Update of ticker symbol %s failed: %s", symbol, err), "danger"})
				}

				_, err = fetchTicker(awssess, db, symbol, exchange.ExchangeMic)
				if err != nil {
					logger.Error().Msgf("Update of ticker symbol %s failed: %s", symbol, err)
					*messages = append(*messages, Message{fmt.Sprintf("Update of ticker symbol %s failed: %s", symbol, err), "danger"})
				} else {
					logger.Info().Str("symbol", symbol).Msg("Update of ticker symbol completed")
					*messages = append(*messages, Message{fmt.Sprintf("Update of ticker symbol %s succeeded", symbol), "success"})
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

	if IsWorkDay && currentTime > "19:00:00" {
		return currentDate
	}

	prevWorkDate := mytime.PriorWorkDate(currentDateTime)
	prevWorkDay := prevWorkDate.Format("2006-01-02")

	return prevWorkDay
}
