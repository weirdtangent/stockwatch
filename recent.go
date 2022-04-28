package main

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

func getRecents(session *sessions.Session, r *http.Request) (*[]string, error) {
	var recents []string

	if session.Values["recents"] != nil {
		recents = session.Values["recents"].([]string)
	}

	return &recents, nil
}

func getRecentsPlusInfo(ctx context.Context, r *http.Request) (*[]RecentPlus, error) {
	session := getSession(r)
	var recentPlus []RecentPlus

	if session.Values["recents"] != nil {
		recents := session.Values["recents"].([]string)
		if len(recents) == 0 {
			return &recentPlus, nil
		}
		symbols := []string{}
		tickers := []Ticker{}
		exchanges := []Exchange{}
		quotes := map[string]yhfinance.YFQuote{}
		// Load up all the tickers and exchanges and fill arrays
		for _, symbol := range recents {
			ticker := Ticker{TickerSymbol: symbol}
			err := ticker.getBySymbol(ctx)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to load recent {symbol}")
				continue
			}
			tickers = append(tickers, ticker)
			symbols = append(symbols, ticker.TickerSymbol)

			if ticker.FavIconS3Key == "" {
				err := ticker.saveFavIcon(ctx)
				if err != nil {
					zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to save favicon for recent {symbol}")
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
	}

	return &recentPlus, nil
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

	// keep only the 5 most recent
	if len(recents) >= 6 {
		recents = recents[:5]
	}
	// prepend latest symbol to front of recents slice
	recents = append([]string{ticker.TickerSymbol}, recents...)
	session.Values["recents"] = recents

	// add/update to recent table
	recent := &Recent{
		ticker.TickerId,
		ticker.MSPerformanceId,
		sql.NullTime{Valid: true, Time: time.Now()},
	}
	recent.createOrUpdate(ctx)

	return &recents, nil
}

func (r *Recent) createOrUpdate(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	if r.TickerId == 0 {
		return nil
	}

	var insert_or_update = "INSERT INTO recent (ticker_id, ms_performance_id) VALUES(?, ?) ON DUPLICATE KEY UPDATE ms_performance_id=?, lastseen_datetime=now()"
	_, err := db.Exec(insert_or_update, r.TickerId, r.MSPerformanceId, r.MSPerformanceId)
	if err != nil {
		log.Warn().Err(err).Str("table_name", "recent").Msg("failed on INSERT OR UPDATE")
		return err
	}
	return nil
}
