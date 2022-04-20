package main

import (
	"context"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Recent struct {
	TickerId         int64  `db:"ticker_id"`
	MSPerformanceId  string `db:"ms_performance_id"`
	LastSeenDatetime string `db:"lastseen_datetime"`
}

func getRecents(session *sessions.Session, r *http.Request) (*[]string, error) {
	// get current list (if any) from session
	var recents []string

	if session.Values["recents"] != nil {
		recents = session.Values["recents"].([]string)
	}

	return &recents, nil
}

func addTickerToRecents(ctx context.Context, r *http.Request, ticker Ticker) (*[]string, error) {
	// get current list (if any) from session
	var recents []string

	session := getSession(r)
	if session.Values["recents"] != nil {
		recents = session.Values["recents"].([]string)
	}

	// if this symbol/exchange is already on the list, remove it so we can add it to the front
	for i, viewed := range recents {
		if viewed == ticker.TickerSymbol {
			recents = append(recents[:i], recents[i+1:]...)
			break
		}
	}

	// keep only the 4 most recent
	if len(recents) >= 5 {
		recents = recents[:4]
	}
	// prepend latest symbol to front of recents slice
	recents = append([]string{ticker.TickerSymbol}, recents...)
	session.Values["recents"] = recents

	// add/update to recent table
	recent := &Recent{
		ticker.TickerId,
		ticker.MSPerformanceId,
		"now()",
	}
	recent.createOrUpdate(ctx)

	return &recents, nil
}

func (r *Recent) createOrUpdate(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if r.TickerId == 0 {
		return nil
	}

	var insert_or_update = "INSERT INTO recent (ticker_id, ms_performance_id) VALUES(?, ?) ON DUPLICATE KEY UPDATE ms_performance_id=?, lastseen_datetime=now()"
	_, err := db.Exec(insert_or_update, r.TickerId, r.MSPerformanceId, r.MSPerformanceId)
	if err != nil {
		logger.Warn().Err(err).
			Str("table_name", "recent").
			Msg("failed on INSERT OR UPDATE")
		return err
	}
	return nil
}
