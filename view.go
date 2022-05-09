package main

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func viewTickerDailyHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata
		sublog := deps.logger
		watcher := checkAuthState(w, r, deps)

		params := mux.Vars(r)
		symbol := params["symbol"]

		// this loads TONS of stuff into webdata
		ticker, err := loadTickerDetails(deps, symbol, 180)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to load ticker details for viewing")
			renderTemplate(w, r, deps, "desktop")
			return
		}

		// add this ticker to recents list
		watcherRecents, err := addToWatcherRecents(deps, watcher, ticker)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to add ticker to recents list")
		}
		webdata["Recents"] = watcherRecents

		renderTemplate(w, r, deps, "view-daily")
	})
}

func viewTickerArticleHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata
		sublog := deps.logger
		watcher := checkAuthState(w, r, deps)

		params := mux.Vars(r)
		symbol := params["symbol"]
		articleEId := params["articleEId"]

		// this loads TONS of stuff into webdata
		ticker, err := loadTickerDetails(deps, symbol, 180)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to load ticker details for viewing")
			renderTemplate(w, r, deps, "desktop")
			return
		}

		// add this ticker to recents list
		watcherRecents, err := addToWatcherRecents(deps, watcher, ticker)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to add ticker to recents list")
		}
		webdata["Recents"] = watcherRecents

		webdata["autoopen_article_encid"] = articleEId

		renderTemplate(w, r, deps, "view-daily")
	})
}

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
			sublog.Error().Err(err).Str("ticker", symbol).Msg("ould not load ticker from yhfinance summary")
			return Ticker{}, err
		}
		sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Msg("timer: yhfinance summary")
		err := ticker.createOrUpdate(deps)
		if err != nil {
			sublog.Error().Err(err).Str("ticker", symbol).Msg("could not update ticker with yhfinance summary")
		}
	} else if !ticker.FetchDatetime.Valid || ticker.FetchDatetime.Time.Add(24*time.Hour).Before(time.Now()) {
		// queue update of ticker from YH
		err := ticker.queueUpdateInfo(deps)
		if err != nil {
			sublog.Error().Err(err).Str("symbol", ticker.TickerSymbol).Msg("failed to queue 'update info' for {symbol}")
		}
	}
	if ticker.TickerSymbol == "" || ticker.TickerId == 0 {
		sublog.Fatal().Interface("ticker", ticker).Msg("ticker object is not saved")
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
		quote, err := fetchTickerQuoteFromYH(deps, ticker.TickerSymbol)
		if err == nil {
			webdata["quote"] = quote
			ticker.updatePriceAndVolume(deps, quote.QuotePrice, quote.QuoteVolume)
		}
		webdata["open"] = true
	}

	// if it is a workday after 4 and we don't have the EOD (or not an EOD from
	// AFTER 4pm) or we don't have the prior workday EOD, get them
	if ticker.needEODs(deps) {
		ticker.queueUpdateEODs(deps)
	}

	tickerUpDowns, _ := ticker.getUpDowns(deps, 90)
	tickerAttributes, _ := ticker.getAttributes(deps)
	tickerSplits, _ := ticker.getSplits(deps)
	_, lastCheckedSince, updatingNewsNow := getLastDoneInfo(deps, "ticker_news", ticker.TickerSymbol)

	// load any recent news
	articles := getArticlesByTicker(deps, ticker, 20, 180)
	webdata["articles"] = articles

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

	webdata["TickerSymbol"] = symbol
	webdata["ticker"] = ticker
	webdata["ticker_description"] = tickerDescription
	webdata["exchange"] = exchange
	webdata["timespan"] = timespan

	// if len(lastTickerDaily) > 0 {
	// 	webdata["lastClose"] = lastTickerDaily[0]
	// 	if len(lastTickerDaily) > 1 {
	// 		webdata["priorClose"] = lastTickerDaily[1]
	// 	}
	// }

	webdata["DiffAmt"] = PriceDiffAmt(ticker.MarketPrevClose, ticker.MarketPrice)
	webdata["DiffPerc"] = PriceDiffPercAmt(ticker.MarketPrevClose, ticker.MarketPrice)

	webdata["ticker_updowns"] = tickerUpDowns
	webdata["ticker_attributes"] = tickerAttributes
	webdata["ticker_splits"] = tickerSplits
	webdata["LastCheckedSince"] = lastCheckedSince
	webdata["UpdatingNewsNow"] = updatingNewsNow
	webdata["TickerFavIconCDATA"] = ticker.getFavIconCDATA(deps)

	return ticker, nil
}
