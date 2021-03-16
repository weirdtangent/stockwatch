package main

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func loadTickerDetails(ctx context.Context, symbol string, timespan int) error {
	logger := log.Ctx(ctx)
	db := ctx.Value("db").(*sqlx.DB)
	messages := ctx.Value("messages").(*[]Message)
	webdata := ctx.Value("webdata").(map[string]interface{})

	// if we have this ticker and today is a weekend or we have today's closing price
	// then we don't need to call APIs and load a bunch of data we already have!
	ticker, err := getTickerBySymbol(ctx, symbol)
	if err != nil || ticker.waitingForClosingPrice(ctx) {
		log.Info().Msg("Check with Yahoo Finance, today is workday and we don't have a closing price yet")

		// get Ticker info
		ticker, err := loadTicker(ctx, symbol)
		if err != nil {
			return err
		}

		// get Quote (live data)
		ticker.getQuote(ctx)

		// get Ticker EODs from API
		updated, err := ticker.updateTickerPrices(ctx)
		if err != nil {
			*messages = append(*messages, Message{fmt.Sprintf("Failed to update End-of-day data for %s", ticker.TickerSymbol), "danger"})
			logger.Warn().Err(err).
				Str("symbol", ticker.TickerSymbol).
				Int64("ticker_id", ticker.TickerId).
				Msg("Failed to update EOD for ticker")
		} else if updated {
			*messages = append(*messages, Message{fmt.Sprintf("End-of-day data updated for %s", ticker.TickerSymbol), "success"})
		}
	}

	// get Ticker_UpDowns
	tickerUpDowns, _ := ticker.getUpDowns(ctx, 90)

	// get Exchange info
	exchange, err := getExchangeById(ctx, ticker.ExchangeId)
	if err != nil {
		return err
	}

	//
	daily, err := getTickerDailyMostRecent(db, ticker.TickerId)
	if err != nil {
		*messages = append(*messages, Message{fmt.Sprintf("No End-of-day data found for %s", ticker.TickerSymbol), "warning"})
	}
	lastTickerDailyMove, err := getLastTickerDailyMove(db, ticker.TickerId)
	if err != nil {
		lastTickerDailyMove = "unknown"
	}

	// load up to last 100 days of EOD data
	ticker_dailies, err := ticker.LoadTickerDailies(db, timespan)
	if err != nil {
		return err
	}

	// load any active watches about this ticker
	webwatches, err := loadWebWatches(db, ticker.TickerId)
	if err != nil {
		return err
	}

	// Build charts
	var lineChartHTML = chartHandlerTickerDailyLine(ctx, ticker, exchange, ticker_dailies, webwatches)
	var klineChartHTML = chartHandlerTickerDailyKLine(ctx, ticker, exchange, ticker_dailies, webwatches)

	webdata["ticker"] = ticker
	webdata["exchange"] = exchange
	webdata["timespan"] = timespan
	webdata["ticker_daily"] = daily
	webdata["ticker_updowns"] = tickerUpDowns
	webdata["last_ticker_daily_move"] = lastTickerDailyMove
	webdata["ticker_dailies"] = TickerDailies{ticker_dailies}
	webdata["watches"] = webwatches
	webdata["lineChart"] = lineChartHTML
	webdata["klineChart"] = klineChartHTML

	return nil
}
