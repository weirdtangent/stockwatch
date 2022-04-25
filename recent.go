package main

import (
	"context"
	"database/sql"
	"net/http"
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
	TickerId        uint64
	TickerSymbol    string
	Exchange        string
	TickerName      string
	CompanyName     string
	LiveQuote       yhfinance.YFQuote
	LastClose       TickerDaily
	PriorClose      TickerDaily
	LastDailyMove   string
	NewsLastUpdated sql.NullTime
	UpdatingNewsNow bool
}

func getRecents(session *sessions.Session, r *http.Request) (*[]string, error) {
	var recents []string

	if session.Values["recents"] != nil {
		recents = session.Values["recents"].([]string)
	}

	return &recents, nil
}

func getRecentsPlusInfo(ctx context.Context, r *http.Request) (*[]RecentPlus, error) {
	webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})
	session := getSession(r)
	var recents []string
	var recentPlus []RecentPlus

	if session.Values["recents"] != nil {
		recents = session.Values["recents"].([]string)
		for _, symbol := range recents {
			ticker := Ticker{TickerSymbol: symbol}
			err := ticker.getBySymbol(ctx)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to load recent {symbol}")
				continue
			}
			exchange := Exchange{ExchangeId: uint64(ticker.ExchangeId)}
			err = exchange.getById(ctx)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to load exchange for recent {symbol}")
				continue
			}
			quote := yhfinance.YFQuote{}
			if isMarketOpen() {
				quote, err = loadTickerQuote(ctx, ticker.TickerSymbol)
				if err != nil {
					zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to load quote for recent {symbol}")
					continue
				}
			}
			lastClose, priorClose := ticker.getLastAndPriorClose(ctx)
			lastDailyMove, _ := getLastTickerDailyMove(ctx, ticker.TickerId)

			newsLastUpdated := sql.NullTime{Valid: false, Time: time.Time{}}
			updatingNewsNow := false
			lastdone := LastDone{Activity: "ticker_news", UniqueKey: ticker.TickerSymbol}
			err = lastdone.getByActivity(ctx)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to get LastDone activity for {symbol}")
			}
			if err == nil && lastdone.LastStatus == "success" {
				newsLastUpdated = sql.NullTime{Valid: true, Time: lastdone.LastDoneDatetime.Time.In(webdata["tzlocation"].(*time.Location))}
				if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerNewsDelay).Before(time.Now()) {
					err = ticker.queueUpdateNews(ctx)
					updatingNewsNow = err == nil
				}
			} else {
				err = ticker.queueUpdateNews(ctx)
				updatingNewsNow = err == nil
			}

			recentPlus = append(recentPlus, RecentPlus{
				TickerId:        ticker.TickerId,
				TickerSymbol:    ticker.TickerSymbol,
				Exchange:        exchange.ExchangeAcronym,
				TickerName:      ticker.TickerName,
				CompanyName:     ticker.CompanyName,
				LiveQuote:       quote,
				LastClose:       *lastClose,
				PriorClose:      *priorClose,
				LastDailyMove:   lastDailyMove,
				NewsLastUpdated: newsLastUpdated,
				UpdatingNewsNow: updatingNewsNow,
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
