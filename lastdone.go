package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

type LastDone struct {
	Activity         string       `db:"activity"`
	UniqueKey        string       `db:"unique_key"`
	LastStatus       string       `db:"last_status"`
	LastDoneDatetime sql.NullTime `db:"lastdone_datetime"`
	CreateDatetime   time.Time    `db:"create_datetime"`
	UpdateDatetime   time.Time    `db:"update_datetime"`
}

// object methods -------------------------------------------------------------
func (ld *LastDone) getByActivity(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM lastdone WHERE activity=? AND unique_key=?", ld.Activity, ld.UniqueKey).StructScan(ld)
	return err
}

// misc -----------------------------------------------------------------------
func getLastDoneInfo(deps *Dependencies, sublog zerolog.Logger, task string, key string) (sql.NullTime, string, bool) {
	sublog = sublog.With().Str("task", task).Logger()

	lastSuccessDatetime := sql.NullTime{Valid: false, Time: time.Time{}}
	lastSuccessSince := "unknown"
	runningTaskNow := false

	lastdone := LastDone{Activity: task, UniqueKey: key}
	err := lastdone.getByActivity(deps)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return lastSuccessDatetime, lastSuccessSince, runningTaskNow
	} else if err != nil {
		sublog.Error().Err(err).Msg("failed to get LastDone activity for {task}")
		return sql.NullTime{}, "", false
	}

	if lastdone.LastDoneDatetime.Valid {
		lastSuccessDatetime = lastdone.LastDoneDatetime
		lastSuccessSince = fmt.Sprintf("%.0f min ago", time.Since(lastSuccessDatetime.Time).Minutes())
	}
	if lastdone.LastStatus == "success" {
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerNewsDelay).Before(time.Now()) {
			return lastSuccessDatetime, lastSuccessSince, runningTaskNow
		} else {
			return lastSuccessDatetime, lastSuccessSince, false
		}
	}
	sublog.Info().Msg("last try failed, queue {task}")

	return lastSuccessDatetime, lastSuccessSince, runningTaskNow
}
