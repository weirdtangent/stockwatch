package main

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/weirdtangent/yhfinance"
)

type Recent struct {
	TickerId         uint64       `db:"ticker_id"`
	MSPerformanceId  string       `db:"ms_performance_id"`
	LastSeenDatetime sql.NullTime `db:"lastseen_datetime"`
}

type RecentPlus struct {
	TickerId           uint64
	TickerSymbol       string
	TickerFavIconCDATA string
	Exchange           string
	TickerName         string
	CompanyName        string
	LiveQuote          yhfinance.YFQuote
	LastClose          TickerDaily
	PriorClose         TickerDaily
	LastDailyMove      string
	LastCheckedNews    sql.NullTime
	UpdatingNewsNow    bool
}

func getWatcherRecents(ctx context.Context, watcher Watcher) []WatcherRecent {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	watcherRecents := make([]WatcherRecent, 0, 30)
	if watcher.WatcherId == 0 {
		zerolog.Ctx(ctx).Info().Msg("watcher not logged in, so no recents are stored")
		return watcherRecents
	}

	rows, err := db.Queryx(`
	  SELECT watcher_recent.*, ticker.ticker_symbol
	  FROM watcher_recent
	  LEFT JOIN ticker USING (ticker_id)
	  WHERE watcher_id=?
	  ORDER BY watcher_recent.update_datetime DESC`, watcher.WatcherId)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Str("table_name", "watcher_recent").Msg("Failed on SELECT")
		return []WatcherRecent{}
	}
	defer rows.Close()

	var watcherRecent WatcherRecent
	for rows.Next() {
		err = rows.StructScan(&watcherRecent)
		if err != nil {
			zerolog.Ctx(ctx).Fatal().Err(err).Str("table_name", "watcher_recent").Msg("Error reading result rows")
			continue
		}
		watcherRecents = append(watcherRecents, watcherRecent)
	}
	if err := rows.Err(); err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Str("table_name", "watch").Msg("Error reading result rows")
	}
	return watcherRecents
}

func getRecentsPlusInfo(ctx context.Context, watcherRecents []WatcherRecent) (*[]RecentPlus, error) {
	var recentPlus []RecentPlus

	symbols := []string{}
	tickers := []Ticker{}
	exchanges := []Exchange{}
	quotes := map[string]yhfinance.YFQuote{}
	// Load up all the tickers and exchanges and fill arrays
	for _, watcherRecent := range watcherRecents {
		zerolog.Ctx(ctx).Info().Msg("checking on watcher_recent" + watcherRecent.TickerSymbol)
		ticker := Ticker{TickerId: watcherRecent.TickerId}
		err := ticker.getById(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to load recent {symbol}")
			continue
		}
		tickers = append(tickers, ticker)
		symbols = append(symbols, ticker.TickerSymbol)

		if ticker.FavIconS3Key == "" {
			err := ticker.queueSaveFavIcon(ctx)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to queue save favicon for recent {symbol}")
			}
		}

		exchange := Exchange{ExchangeId: uint64(ticker.ExchangeId)}
		err = exchange.getById(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to load exchange for recent {symbol}")
			continue
		}
		exchanges = append(exchanges, exchange)

		quotes[ticker.TickerSymbol] = yhfinance.YFQuote{}
	}

	// if market open, get all quotes in one call
	if isMarketOpen() {
		var err error
		quotes, err = loadMultiTickerQuotes(ctx, symbols)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Str("symbols", strings.Join(symbols, ",")).Msg("failed to load quote for recent {symbol}")
			return &recentPlus, err
		}
	} else {
		// if it is a workday after 4 and we don't have the EOD (or not an EOD from
		// AFTER 4pm) or we don't have the prior workday EOD, get them
		for _, ticker := range tickers {
			if ticker.needEODs(ctx) {
				loadTickerEODs(ctx, ticker)
			}
		}
	}

	// build recentPlus array
	for n, symbol := range symbols {
		quote, ok := quotes[symbol]
		if !ok {
			continue
		}
		ticker := tickers[n]
		exchange := exchanges[n]

		lastTickerDaily, _ := getLastTickerDaily(ctx, ticker.TickerId)
		lastDailyMove, _ := getLastTickerDailyMove(ctx, ticker.TickerId)

		lastCheckedNews, updatingNewsNow := getNewsLastUpdated(ctx, ticker)

		recentPlus = append(recentPlus, RecentPlus{
			TickerId:           ticker.TickerId,
			TickerSymbol:       ticker.TickerSymbol,
			TickerFavIconCDATA: ticker.getFavIconCDATA(ctx),
			Exchange:           exchange.ExchangeAcronym,
			TickerName:         ticker.TickerName,
			CompanyName:        ticker.CompanyName,
			LiveQuote:          quote,
			LastClose:          lastTickerDaily[0],
			PriorClose:         lastTickerDaily[1],
			LastDailyMove:      lastDailyMove,
			LastCheckedNews:    lastCheckedNews,
			UpdatingNewsNow:    updatingNewsNow,
		})
	}

	return &recentPlus, nil
}

func addTickerToRecents(ctx context.Context, watcher Watcher, ticker Ticker) ([]WatcherRecent, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	watcherRecent := WatcherRecent{0, watcher.WatcherId, ticker.TickerId, ticker.TickerSymbol, false, sql.NullTime{Valid: true, Time: time.Now()}, sql.NullTime{Valid: true, Time: time.Now()}}
	err := watcherRecent.create(ctx)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Msg("failed to save to watcher_recent")
	}

	// add/update to recent table
	recent := &Recent{
		ticker.TickerId,
		ticker.MSPerformanceId,
		sql.NullTime{Valid: true, Time: time.Now()},
	}
	recent.createOrUpdate(ctx)

	var count int32
	err = db.QueryRowx("SELECT count(*) FROM watcher_recent WHERE watcher_id=?", watcher.WatcherId).Scan(&count)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("table_name", "watcher_recent").Msg("failed on SELECT")
	} else {
		if count > maxRecentCount {
			_, err := db.Exec("DELETE FROM watcher_recent WHERE watcher_id=? ORDER BY update_datetime LIMIT ?", watcher.WatcherId, count-maxRecentCount)
			if err != nil {
				zerolog.Ctx(ctx).Warn().Err(err).Str("table_name", "watcher_recent").Msg("failed on DELETE")
			}
		}
	}

	return getWatcherRecents(ctx, watcher), err
}

func (r *Recent) createOrUpdate(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	if r.TickerId == 0 {
		return nil
	}

	var insert_or_update = "INSERT INTO recent (ticker_id, ms_performance_id) VALUES(?, ?) ON DUPLICATE KEY UPDATE ms_performance_id=?, lastseen_datetime=now()"
	_, err := db.Exec(insert_or_update, r.TickerId, r.MSPerformanceId, r.MSPerformanceId)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("table_name", "recent").Msg("failed on INSERT OR UPDATE")
		return err
	}
	return nil
}
