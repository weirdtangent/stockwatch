package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

func loadTickerDetails(ctx context.Context, symbol string, timespan int) (Ticker, error) {
	messages := ctx.Value(ContextKey("messages")).(*[]Message)
	webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})

	// load ticker from yhfinance if we don't have it or what we have is > 24 hours old
	ticker := Ticker{TickerSymbol: symbol}
	err := ticker.getBySymbol(ctx)
	if err != nil || !ticker.FetchDatetime.Valid || time.Now().Add(time.Hour).Before(ticker.FetchDatetime.Time) || !skipLocalTickerInfo {
		ticker, err = fetchTickerInfo(ctx, symbol)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Str("ticker", symbol).Msg("Fatal: could not load ticker info from source. Redirect back to desktop?")
			return Ticker{}, err
		}
		*messages = append(*messages, Message{"Company/Symbol data updated", "success"})
	}

	tickerDescription, _ := getTickerDescriptionByTickerId(ctx, ticker.TickerId)

	// get Exchange info
	exchange := Exchange{ExchangeId: uint64(ticker.ExchangeId)}
	err = exchange.getById(ctx)
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("Fatal: could not load exchange info. Redirect back to desktop?")
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
	webwatches, _ := loadWebWatches(ctx, ticker.TickerId)

	// load any recent news
	articles, _ := getArticlesByTicker(ctx, ticker.TickerId)
	if len(articles) > 0 {
		webdata["articles"] = articles
		for _, article := range articles {
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
				zerolog.Ctx(ctx).Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateNews")
			}
		}
	} else {
		err = ticker.queueUpdateNews(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateNews")
		}
	}

	// schedule to update ticker financials
	lastdone = LastDone{Activity: "ticker_financials", UniqueKey: ticker.TickerSymbol}
	err = lastdone.getByActivity(ctx)
	if err == nil && lastdone.LastStatus == "success" {
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerFinancialsDelay).Before(time.Now()) {
			err = ticker.queueUpdateFinancials(ctx)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateFinancials")
			}
		}
	} else {
		err = ticker.queueUpdateFinancials(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateFinancials")
		}
	}

	// Build charts
	var lineChartHTML = chartHandlerTickerDailyLine(ctx, ticker, &exchange, ticker_dailies, webwatches)
	var klineChartHTML = chartHandlerTickerDailyKLine(ctx, ticker, &exchange, ticker_dailies, webwatches)

	// get financials
	qtrBarStrs, qtrBarValues, _ := ticker.GetFinancials(ctx, "Quarterly", "bar", 0)
	annBarStrs, annBarValues, _ := ticker.GetFinancials(ctx, "Annual", "bar", 0)
	var qtrBarChartHTML = chartHandlerFinancialsBar(ctx, ticker, &exchange, qtrBarStrs, qtrBarValues)
	var annBarChartHTML = chartHandlerFinancialsBar(ctx, ticker, &exchange, annBarStrs, annBarValues)

	qtrLineStrs, qtrLineValues, _ := ticker.GetFinancials(ctx, "Quarterly", "line", 0)
	annLineStrs, annLineValues, _ := ticker.GetFinancials(ctx, "Annual", "line", 0)
	var qtrLineChartHTML = chartHandlerFinancialsLine(ctx, ticker, &exchange, qtrLineStrs, qtrLineValues, 0)
	var annLineChartHTML = chartHandlerFinancialsLine(ctx, ticker, &exchange, annLineStrs, annLineValues, 0)

	qtrPercStrs, qtrPercValues, _ := ticker.GetFinancials(ctx, "Quarterly", "line", 1)
	annPercStrs, annPercValues, _ := ticker.GetFinancials(ctx, "Annual", "line", 1)
	var qtrPercChartHTML = chartHandlerFinancialsLine(ctx, ticker, &exchange, qtrPercStrs, qtrPercValues, 1)
	var annPercChartHTML = chartHandlerFinancialsLine(ctx, ticker, &exchange, annPercStrs, annPercValues, 1)

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
	webdata["qtrLineChart"] = qtrLineChartHTML
	webdata["annLineChart"] = annLineChartHTML
	webdata["qtrPercChart"] = qtrPercChartHTML
	webdata["annPercChart"] = annPercChartHTML

	return ticker, nil
}
