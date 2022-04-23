package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func loadTickerDetails(ctx context.Context, symbol string, timespan int) (Ticker, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	messages := ctx.Value(ContextKey("messages")).(*[]Message)
	webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})

	// if we have this ticker and today is a weekend or we have today's closing price
	// then we don't need to call APIs and load a bunch of data we already have!
	ticker, err := getTickerBySymbol(ctx, symbol)

	// if not there, or more than 24 hours old, hit API
	if err != nil || Over24HoursUTC(ticker.FetchDatetime) {
		// get Ticker info
		ticker, err = loadTicker(ctx, symbol)
		if err != nil {
			logger.Error().Err(err).Str("ticker", symbol).Msg("Fatal: could not load ticker info. Redirect back to desktop?")
			return Ticker{}, err
		}
		*messages = append(*messages, Message{"Company/Symbol data updated", "success"})
	}

	tickerDescription, _ := getTickerDescriptionByTickerId(ctx, ticker.TickerId)

	// get Exchange info
	exchange, err := getExchangeById(ctx, ticker.ExchangeId)
	if err != nil {
		logger.Error().Err(err).Str("ticker", symbol).Int64("exchange_id", ticker.ExchangeId).Msg("Fatal: could not load exchange info. Redirect back to desktop?")
		return Ticker{}, err
	}

	// if the market is open, lets get a live quote
	if isMarketOpen() {
		quote, err := loadTickerQuote(ctx, ticker.TickerSymbol)
		if err == nil {
			webdata["quote"] = quote
			*messages = append(*messages, Message{"Live quote data updated", "success"})
		}
		webdata["open"] = true
	}

	// if it is a workday after 4 and we don't have the EOD (or not an EOD from
	// AFTER 4pm) or we don't have the prior workday EOD, get them
	if ticker.needEODs(ctx) {
		loadTickerEODs(ctx, ticker)
		*messages = append(*messages, Message{"Historical data updated", "success"})
	}

	// get Ticker_UpDowns
	tickerUpDowns, _ := ticker.getUpDowns(ctx, 90)

	// get Ticker_Attributes
	tickerAttributes, _ := ticker.getAttributes(ctx)

	// get Ticker_Splits
	tickerSplits, _ := ticker.getSplits(ctx)

	lastClose, priorClose := ticker.getLastAndPriorClose(ctx)
	lastTickerDailyMove, _ := getLastTickerDailyMove(ctx, ticker.TickerId)

	// load up to last 100 days of EOD data
	ticker_dailies, _ := ticker.getTickerEODs(ctx, timespan)

	// load any active watches about this ticker
	webwatches, _ := loadWebWatches(db, ticker.TickerId)

	// load any recent news
	articles, _ := getArticlesByTicker(ctx, ticker.TickerId)
	if len(*articles) > 0 {
		webdata["articles"] = articles
		for _, article := range *articles {
			key := fmt.Sprintf("_source%d-id%s-body_template", article.SourceId, article.ExternalId)
			webdata[key] = article
		}
	}

	// schedule to update ticker news
	lastdone := LastDone{Activity: "ticker_news", UniqueKey: ticker.TickerSymbol}
	err = lastdone.getByActivity(ctx)
	if err == nil && lastdone.LastStatus == "success" {
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerNewsDelay).Before(time.Now()) {
			err = ticker.queueUpdateNews(ctx)
			if err != nil {
				logger.Error().Err(err).Str("ticker", symbol).Int64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateNews")
			}
		}
	} else {
		err = ticker.queueUpdateNews(ctx)
		if err != nil {
			logger.Error().Err(err).Str("ticker", symbol).Int64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateNews")
		}
	}

	// schedule to update ticker financials
	lastdone = LastDone{Activity: "ticker_financials", UniqueKey: ticker.TickerSymbol}
	err = lastdone.getByActivity(ctx)
	if err == nil && lastdone.LastStatus == "success" {
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerFinancialsDelay).Before(time.Now()) {
			err = ticker.queueUpdateNews(ctx)
			if err != nil {
				logger.Error().Err(err).Str("ticker", symbol).Int64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateFinancials")
			}
		}
	} else {
		err = ticker.queueUpdateNews(ctx)
		if err != nil {
			logger.Error().Err(err).Str("ticker", symbol).Int64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateFinancials")
		}
	}

	// get financials
	quarterStrs, qtrBarValues, _ := ticker.GetFinancials(ctx, "bar", "quarterly")
	annualStrs, annBarValues, _ := ticker.GetFinancials(ctx, "bar", "annual")

	// Build charts
	var lineChartHTML = chartHandlerTickerDailyLine(ctx, ticker, exchange, ticker_dailies, webwatches)
	var klineChartHTML = chartHandlerTickerDailyKLine(ctx, ticker, exchange, ticker_dailies, webwatches)
	var qtrBarChartHTML = chartHandlerFinancialsBar(ctx, ticker, exchange, quarterStrs, qtrBarValues)
	var annBarChartHTML = chartHandlerFinancialsBar(ctx, ticker, exchange, annualStrs, annBarValues)

	webdata["ticker"] = ticker
	webdata["ticker_description"] = tickerDescription
	webdata["exchange"] = exchange
	webdata["timespan"] = timespan
	webdata["lastClose"] = lastClose
	webdata["priorClose"] = priorClose
	webdata["ticker_updowns"] = tickerUpDowns
	webdata["ticker_attributes"] = tickerAttributes
	webdata["ticker_splits"] = tickerSplits
	webdata["last_ticker_daily_move"] = lastTickerDailyMove
	webdata["ticker_dailies"] = TickerDailies{ticker_dailies}
	webdata["watches"] = webwatches
	webdata["lineChart"] = lineChartHTML
	webdata["klineChart"] = klineChartHTML
	webdata["qtrBarChart"] = qtrBarChartHTML
	webdata["annBarChart"] = annBarChartHTML

	return *ticker, nil
}
