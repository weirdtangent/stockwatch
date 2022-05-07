package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/weirdtangent/yhfinance"
)

type Recent struct {
	TickerId         uint64 `db:"ticker_id"`
	EId              string
	MSPerformanceId  string    `db:"ms_performance_id"`
	LastSeenDatetime time.Time `db:"lastseen_datetime"`
}

type RecentPlus struct {
	TickerId           uint64
	EId                string
	TickerSymbol       string
	TickerFavIconCDATA string
	Exchange           string
	TickerName         string
	CompanyName        string
	LiveQuote          yhfinance.YHQuote
	LastClose          TickerDaily
	PriorClose         TickerDaily
	DiffAmt            float64
	DiffPerc           float64
	LastDailyMove      string
	LastCheckedNews    sql.NullTime
	LastCheckedSince   string
	UpdatingNewsNow    bool
	Locked             bool
	Articles           []WebArticle
}

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

func getRecentsPlusInfo(deps *Dependencies, watcherRecents []WatcherRecent) []RecentPlus {
	sublog := deps.logger

	var recentPlus []RecentPlus

	symbols := []string{}
	tickers := []Ticker{}
	locked := []bool{}
	exchanges := []Exchange{}
	quotes := map[string]yhfinance.YHQuote{}
	// Load up all the tickers and exchanges and fill arrays
	for _, watcherRecent := range watcherRecents {
		ticker := Ticker{TickerId: watcherRecent.TickerId}
		err := ticker.getById(deps)
		if err != nil {
			sublog.Warn().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to load recent {symbol}")
			continue
		}
		tickers = append(tickers, ticker)
		symbols = append(symbols, ticker.TickerSymbol)
		locked = append(locked, watcherRecent.Locked)

		if ticker.FavIconS3Key == "" {
			err := ticker.queueSaveFavIcon(deps)
			if err != nil {
				sublog.Warn().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to queue save favicon for recent {symbol}")
			}
		}

		exchange := Exchange{ExchangeId: uint64(ticker.ExchangeId)}
		err = exchange.getById(deps)
		if err != nil {
			sublog.Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to load exchange for recent {symbol}")
			continue
		}
		exchanges = append(exchanges, exchange)

		quotes[ticker.TickerSymbol] = yhfinance.YHQuote{}
	}

	// if market open, get all quotes in one call
	if isMarketOpen() {
		var err error
		quotes, err = loadMultiTickerQuotes(deps, symbols)
		if err != nil {
			sublog.Error().Err(err).Str("symbols", strings.Join(symbols, ",")).Msg("failed to load quote for recent {symbol}")
			return recentPlus
		}
	} else {
		// if it is a workday after 4 and we don't have the EOD (or not an EOD from
		// AFTER 4pm) or we don't have the prior workday EOD, get them
		for _, ticker := range tickers {
			if ticker.needEODs(deps) {
				err := loadTickerEODsFromYH(deps, ticker)
				if err != nil {
					sublog.Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to get ticker eods for {symbol}")
					return recentPlus
				}
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

		lastTickerDaily, _ := getLastTickerDaily(deps, ticker.TickerId)
		lastDailyMove, _ := getLastTickerDailyMove(deps, ticker.TickerId)

		_, lastCheckedSince, updatingNewsNow := getLastDoneInfo(deps, "ticker_news", ticker.TickerSymbol)

		// load any recent news
		tickerArticles := getArticlesByTicker(deps, ticker, 5, 7)
		for n := range tickerArticles {
			if tickerArticles[n].ArticleURL == "" {
				tickerArticles[n].ArticleURL = fmt.Sprintf("/view/%s/%s",
					ticker.TickerSymbol,
					tickerArticles[n].EId)
			} else {
				tickerArticles[n].ExternalURL = true
			}
		}

		recentPlus = append(recentPlus, RecentPlus{
			TickerId:           ticker.TickerId,
			TickerSymbol:       ticker.TickerSymbol,
			TickerFavIconCDATA: ticker.getFavIconCDATA(deps),
			Exchange:           exchange.ExchangeAcronym,
			TickerName:         ticker.TickerName,
			CompanyName:        ticker.CompanyName,
			LiveQuote:          quote,
			LastClose:          lastTickerDaily[0],
			PriorClose:         lastTickerDaily[1],
			DiffAmt:            PriceDiffAmt(lastTickerDaily[1].ClosePrice, lastTickerDaily[0].ClosePrice),
			DiffPerc:           PriceDiffPercAmt(lastTickerDaily[1].ClosePrice, lastTickerDaily[0].ClosePrice),
			LastDailyMove:      lastDailyMove,
			LastCheckedSince:   lastCheckedSince,
			UpdatingNewsNow:    updatingNewsNow,
			Locked:             locked[n],
			Articles:           tickerArticles,
		})
	}

	return recentPlus
}

func addToWatcherRecents(deps *Dependencies, watcher Watcher, ticker Ticker) ([]WatcherRecent, error) {
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

func (r *Recent) createOrUpdate(deps *Dependencies) error {
	db := deps.db

	if r.TickerId == 0 {
		return nil
	}

	var insert_or_update = "INSERT INTO recent (ticker_id, ms_performance_id) VALUES(?, ?) ON DUPLICATE KEY UPDATE ms_performance_id=?, lastseen_datetime=now()"
	_, err := db.Exec(insert_or_update, r.TickerId, r.MSPerformanceId, r.MSPerformanceId)
	return err
}
