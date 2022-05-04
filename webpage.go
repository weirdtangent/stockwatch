package main

import (
	"fmt"
	"time"
)

func loadTickerDetails(deps *Dependencies, symbol string, timespan int) (Ticker, error) {
	sublog := deps.logger
	webdata := deps.webdata

	// load ticker from yhfinance if we don't have it or what we have is > 24 hours old
	ticker := Ticker{TickerSymbol: symbol}
	err := ticker.getBySymbol(deps)
	if err != nil || skipLocalTickerInfo {
		start := time.Now()
		ticker, err = fetchTickerInfoFromYH(deps, symbol)
		if err != nil {
			sublog.Error().Err(err).Str("ticker", symbol).Msg("Fatal: could not load ticker info from source. Redirect back to desktop?")
			return Ticker{}, err
		}
		sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Str("action", "YHfinance get-summary").Msg("timer")
	} else if !ticker.FetchDatetime.Valid || ticker.FetchDatetime.Time.Add(24*time.Hour).Before(time.Now()) {
		// queue update of ticker from YH
		err := ticker.queueUpdateInfo(deps)
		if err != nil {
			sublog.Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to queue 'update info' for {symbol}")
		}
	}

	tickerDescription, _ := getTickerDescriptionByTickerId(deps, ticker.TickerId)

	// get Exchange info
	exchange := Exchange{ExchangeId: uint64(ticker.ExchangeId)}
	err = exchange.getById(deps)
	if err != nil {
		sublog.Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("Fatal: could not load exchange info. Redirect back to desktop?")
		return Ticker{}, err
	}

	// if the market is open, lets get a live quote
	if isMarketOpen() {
		start := time.Now()
		sublog.Debug().Msg("market is open, lets get live quote from YH")
		quote, err := fetchTickerQuoteFromYH(deps, ticker.TickerSymbol)
		if err == nil {
			webdata["quote"] = quote
		}
		webdata["open"] = true
		sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Str("action", "YHfinance get live quote").Msg("timer")
	}

	// if it is a workday after 4 and we don't have the EOD (or not an EOD from
	// AFTER 4pm) or we don't have the prior workday EOD, get them
	if ticker.needEODs(deps) {
		start := time.Now()
		sublog.Debug().Msg("going to get EODs from YH")
		loadTickerEODsFromYH(deps, ticker)
		sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Str("action", "build charts").Msg("timer")
	}

	// get Ticker_UpDowns
	tickerUpDowns, _ := ticker.getUpDowns(deps, 90)
	tickerAttributes, _ := ticker.getAttributes(deps)
	tickerSplits, _ := ticker.getSplits(deps)
	lastTickerDaily, _ := getLastTickerDaily(deps, ticker.TickerId)
	lastTickerDailyMove, _ := getLastTickerDailyMove(deps, ticker.TickerId)
	lastCheckedNews, updatingNewsNow := getNewsLastUpdated(deps, ticker)

	// load up to last 100 days of EOD data
	ticker_dailies, _ := ticker.getTickerEODs(deps, timespan)

	// load any active watches about this ticker
	webwatches, _ := loadWebWatches(deps, ticker.TickerId)

	// load any recent news
	articles, _ := getArticlesByTicker(deps, ticker.TickerId, 20, 180)
	if len(articles) > 0 {
		webdata["articles"] = articles
		for _, article := range articles {
			key := fmt.Sprintf("_source%d-id%s", article.SourceId, article.ExternalId)
			webdata[key] = article
		}
	}

	// schedule to update ticker news
	lastdone := LastDone{Activity: "ticker_news", UniqueKey: ticker.TickerSymbol}
	err = lastdone.getByActivity(deps)
	if err == nil && lastdone.LastStatus == "success" {
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerNewsDelay).Before(time.Now()) {
			err = ticker.queueUpdateNews(deps)
			if err != nil {
				sublog.Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateNews")
			}
		}
	} else {
		err = ticker.queueUpdateNews(deps)
		if err != nil {
			sublog.Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateNews")
		}
	}

	// schedule to update ticker financials
	lastdone = LastDone{Activity: "ticker_financials", UniqueKey: ticker.TickerSymbol}
	err = lastdone.getByActivity(deps)
	if err == nil && lastdone.LastStatus == "success" {
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerFinancialsDelay).Before(time.Now()) {
			err = ticker.queueUpdateFinancials(deps)
			if err != nil {
				sublog.Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateFinancials")
			}
		}
	} else {
		err = ticker.queueUpdateFinancials(deps)
		if err != nil {
			sublog.Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateFinancials")
		}
	}

	start := time.Now()
	// Build charts
	var lineChartHTML = chartHandlerTickerDailyLine(deps, ticker, &exchange, ticker_dailies, webwatches)
	var klineChartHTML = chartHandlerTickerDailyKLine(deps, ticker, &exchange, ticker_dailies, webwatches)

	// get financials
	qtrBarStrs, qtrBarValues, _ := ticker.GetFinancials(deps, "Quarterly", "bar", 0)
	annBarStrs, annBarValues, _ := ticker.GetFinancials(deps, "Annual", "bar", 0)
	var qtrBarChartHTML = chartHandlerFinancialsBar(deps, ticker, &exchange, qtrBarStrs, qtrBarValues)
	var annBarChartHTML = chartHandlerFinancialsBar(deps, ticker, &exchange, annBarStrs, annBarValues)

	qtrLineStrs, qtrLineValues, _ := ticker.GetFinancials(deps, "Quarterly", "line", 0)
	annLineStrs, annLineValues, _ := ticker.GetFinancials(deps, "Annual", "line", 0)
	var qtrLineChartHTML = chartHandlerFinancialsLine(deps, ticker, &exchange, qtrLineStrs, qtrLineValues, 0)
	var annLineChartHTML = chartHandlerFinancialsLine(deps, ticker, &exchange, annLineStrs, annLineValues, 0)

	qtrPercStrs, qtrPercValues, _ := ticker.GetFinancials(deps, "Quarterly", "line", 1)
	annPercStrs, annPercValues, _ := ticker.GetFinancials(deps, "Annual", "line", 1)
	var qtrPercChartHTML = chartHandlerFinancialsLine(deps, ticker, &exchange, qtrPercStrs, qtrPercValues, 1)
	var annPercChartHTML = chartHandlerFinancialsLine(deps, ticker, &exchange, annPercStrs, annPercValues, 1)
	sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Str("action", "build charts").Msg("timer")

	// localTz, err := time.LoadLocation(webdata["TZLocation"].(string))
	// if err != nil {
	localTz, _ := time.LoadLocation("UTC")
	// }

	webdata["TickerSymbol"] = symbol
	webdata["ticker"] = ticker
	webdata["ticker_description"] = tickerDescription
	webdata["exchange"] = exchange
	webdata["timespan"] = timespan
	webdata["lastClose"] = lastTickerDaily[0]
	webdata["priorClose"] = lastTickerDaily[1]
	webdata["DiffAmt"] = PriceDiffAmt(lastTickerDaily[1].ClosePrice, lastTickerDaily[0].ClosePrice)
	webdata["DiffPerc"] = PriceDiffPercAmt(lastTickerDaily[1].ClosePrice, lastTickerDaily[0].ClosePrice)
	webdata["ticker_updowns"] = tickerUpDowns
	webdata["ticker_attributes"] = tickerAttributes
	webdata["ticker_splits"] = tickerSplits
	webdata["last_ticker_daily_move"] = lastTickerDailyMove
	webdata["ticker_dailies"] = TickerDailies{ticker_dailies}
	webdata["LastCheckedNews"] = lastCheckedNews.Time.In(localTz)
	webdata["UpdatingNewsNow"] = updatingNewsNow
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
