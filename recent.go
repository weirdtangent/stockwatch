package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

type Recent struct {
	TickerId         uint64 `db:"ticker_id"`
	EId              string
	MSPerformanceId  string    `db:"ms_performance_id"`
	LastSeenDatetime time.Time `db:"lastseen_datetime"`
}

// object methods -------------------------------------------------------------

func (r *Recent) createOrUpdate(deps *Dependencies) error {
	db := deps.db

	if r.TickerId == 0 {
		return nil
	}

	var insert_or_update = "INSERT INTO recent (ticker_id, ms_performance_id) VALUES(?, ?) ON DUPLICATE KEY UPDATE ms_performance_id=?, lastseen_datetime=now()"
	_, err := db.Exec(insert_or_update, r.TickerId, r.MSPerformanceId, r.MSPerformanceId)
	return err
}

// misc -----------------------------------------------------------------------

func getWatcherRecents(deps *Dependencies, watcher Watcher) []WatcherRecent {
	db := deps.db
	sublog := deps.logger.With().Str("watcherEid", encryptId(deps, "watcher", watcher.WatcherId)).Logger()

	watcherRecents := make([]WatcherRecent, 0, 30)
	if watcher.WatcherId == 0 {
		return watcherRecents
	}

	rows, err := db.Queryx(`
	  SELECT watcher_recent.*, ticker.ticker_symbol
	  FROM watcher_recent
	  LEFT JOIN ticker USING (ticker_id)
	  WHERE watcher_id=?
	  ORDER BY watcher_recent.update_datetime DESC`, watcher.WatcherId)
	if err != nil {
		sublog.Error().Err(err).Msg("error with query")
		return []WatcherRecent{}
	}

	defer rows.Close()
	var watcherRecent WatcherRecent
	for rows.Next() {
		err = rows.StructScan(&watcherRecent)
		if err != nil {
			sublog.Error().Err(err).Msg("error reading row")
			continue
		}
		watcherRecents = append(watcherRecents, watcherRecent)
	}
	if err := rows.Err(); err != nil {
		sublog.Error().Err(err).Msg("error reading rows")
	}
	return watcherRecents
}

func addTickerToWatcherRecents(deps *Dependencies, sublog zerolog.Logger, watcher Watcher, ticker Ticker) ([]WatcherRecent, error) {
	db := deps.db

	if watcher.WatcherId == 0 {
		return []WatcherRecent{}, fmt.Errorf("not adding recents for watcherId 0")
	}
	watcherRecent, err := getWatcherRecent(deps, watcher, ticker)
	if err != nil {
		watcherRecent = WatcherRecent{0, "", watcher.WatcherId, ticker.TickerId, ticker.TickerSymbol, false, time.Now(), time.Now()}
		err = watcherRecent.create(deps)
		if err != nil {
			return getWatcherRecents(deps, watcher), err
		}

		// if at max already, need to delete an unlocked one before allowing another
		var count int32
		err = db.QueryRowx("SELECT count(*) FROM watcher_recent WHERE watcher_id=?", watcher.WatcherId).Scan(&count)
		if err != nil {
			return getWatcherRecents(deps, watcher), err
		} else {
			if count >= maxRecentCount {
				_, err := db.Exec("DELETE FROM watcher_recent WHERE watcher_id=? AND locked=false ORDER BY update_datetime LIMIT ?", watcher.WatcherId, count-maxRecentCount)
				if err != nil && errors.Is(err, sql.ErrNoRows) {
					return getWatcherRecents(deps, watcher), nil
				}
				if err != nil {
					return getWatcherRecents(deps, watcher), err
				}
			}
		}
	} else {
		err = watcherRecent.update(deps, watcher, ticker)
		if err != nil {
			return getWatcherRecents(deps, watcher), err
		}
	}

	// add/update to recent table
	recent := Recent{ticker.TickerId, "", ticker.MSPerformanceId, time.Now()}
	recent.createOrUpdate(deps)

	return getWatcherRecents(deps, watcher), err
}

func isWatcherRecent(deps *Dependencies, sublog zerolog.Logger, watcher Watcher, ticker Ticker) (bool, error) {
	db := deps.db

	count := 0
	err := db.QueryRowx(`SELECT COUNT(*) FROM watcher_recent WHERE watcher_id=? and ticker_id=?`, watcher.WatcherId, ticker.TickerId).Scan(&count)
	return count == 1, err
}

func getWatcherRecent(deps *Dependencies, watcher Watcher, ticker Ticker) (WatcherRecent, error) {
	db := deps.db

	recent := WatcherRecent{}
	err := db.QueryRowx(`SELECT * FROM watcher_recent WHERE watcher_id=? and ticker_id=?`, watcher.WatcherId, ticker.TickerId).StructScan(&recent)
	return recent, err
}

func removeFromWatcherRecents(deps *Dependencies, watcher Watcher, ticker Ticker) error {
	db := deps.db

	_, err := db.Exec("DELETE FROM watcher_recent WHERE watcher_id=? AND ticker_id=? AND locked=false", watcher.WatcherId, ticker.TickerId)
	return err
}
