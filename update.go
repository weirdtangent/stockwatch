package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	//"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func updateHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		messages := ctx.Value(ContextKey("messages")).(*[]Message)

		if ok := checkAuthState(w, r); !ok {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		params := mux.Vars(r)
		action := params["action"]

		var err error

		switch action {
		case "movers":
			err = loadMovers(ctx)
			if err != nil {
				*messages = append(*messages, Message{fmt.Sprintf("pulling latest Morningstar Movers failed: %s", err.Error()), "danger"})
			} else {
				*messages = append(*messages, Message{"pulled latest Morningstar Movers", "success"})
			}
		case "msnews":
			query := r.FormValue("q")
			tickerId, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
			if len(query) < 1 {
				*messages = append(*messages, Message{"no query string found", "danger"})
			} else if err != nil {
				*messages = append(*messages, Message{"ticker_id not provided or invalid", "danger"})
			} else {
				err = loadMSNews(ctx, query, tickerId)
				if err != nil {
					*messages = append(*messages, Message{fmt.Sprintf("pulling Morningstar News for %s failed: %s", query, err.Error()), "danger"})
				} else {
					*messages = append(*messages, Message{fmt.Sprintf("pulled Morningstar News for %s", query), "success"})
				}
			}
		case "bbnews":
			query := r.FormValue("q")
			if len(query) < 1 {
				*messages = append(*messages, Message{"no query string found", "danger"})
			} else {
				err = loadBBNewsArticles(ctx, query)
				if err != nil {
					*messages = append(*messages, Message{fmt.Sprintf("pulling latest Bloomberg Market News failed: %s", err.Error()), "danger"})
				} else {
					*messages = append(*messages, Message{"pulled latest Bloomberg Market News", "success"})
				}
			}
		default:
			err = errors.New("unknown update action")
			logger.Error().Str("action", action).Msg("Unknown update action")
			*messages = append(*messages, Message{fmt.Sprintf("unknown update action: %s", action), "danger"})
		}

		if err == nil {
			logger.Info().Msgf("Update operation ended normally")
		}
		renderTemplateDefault(w, r, "update")
	})
}

// func mostRecentPricesAvailable() string {
// 	EasternTZ, err := time.LoadLocation("America/New_York")
// 	if err != nil {
// 		log.Error().Err(err).
// 			Msg("Failed to get timezone")
// 		return "1970-01-01"
// 	}
// 	currentDateTime := time.Now().In(EasternTZ)
// 	currentTime := currentDateTime.Format("15:04:05")
// 	currentDate := currentDateTime.Format("2006-01-02")
// 	IsWorkDay := mytime.IsWorkday(currentDateTime)

// 	if IsWorkDay && currentTime > "16:00:00" {
// 		return currentDate
// 	}

// 	prevWorkDate := mytime.PriorWorkDate(currentDateTime)
// 	prevWorkDay := prevWorkDate.Format("2006-01-02")

// 	return prevWorkDay
// }
