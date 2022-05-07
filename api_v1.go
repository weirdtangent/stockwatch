package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type jsonResponseData struct {
	ApiVersion string                 `json:"api_version"`
	Endpoint   string                 `json:"endpoint"`
	Success    bool                   `json:"success"`
	Message    string                 `json:"message"`
	Data       map[string]interface{} `json:"data"`
}

func apiV1Handler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		watcher := checkAuthState(w, r, deps)
		sublog := deps.logger

		w.Header().Add("Content-Type", "application/json")

		// get already inplace nonce from the current page and use it so our answer is allowed
		reqHeader := r.Header
		nonce := reqHeader.Get("X-Nonce")
		deps.nonce = nonce // we don't use webdata in api, so no need to fix that one

		params := mux.Vars(r)
		endpoint := params["endpoint"]

		jsonResponse := jsonResponseData{ApiVersion: "0.1.0", Endpoint: endpoint, Success: false, Data: make(map[string]interface{})}
		newlog := sublog.With().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", jsonResponse.Endpoint).Logger()
		sublog = &newlog

		switch endpoint {
		case "version":
			jsonResponse.Success = true
			jsonResponse.Message = "ok"

		case "quotes":
			symbolStr := r.FormValue("symbols")
			apiQuotes(deps, symbolStr, &jsonResponse)

		case "recents":
			if r.FormValue("remove") != "" {
				removeStr := r.FormValue("remove")
				apiRecents(deps, watcher, "remove", removeStr, &jsonResponse)
			} else if r.FormValue("lock") != "" {
				lockStr := r.FormValue("lock")
				apiRecents(deps, watcher, "lock", lockStr, &jsonResponse)
			} else if r.FormValue("unlock") != "" {
				unlockStr := r.FormValue("unlock")
				apiRecents(deps, watcher, "unlock", unlockStr, &jsonResponse)
			}

		case "chart":
			chart := r.FormValue("chart")
			symbol := r.FormValue("symbol")
			timespan, err := strconv.Atoi(r.FormValue("timespan"))
			if err != nil {
				jsonResponse.Success = false
				jsonResponse.Message = "Failure: invalid timespan"
				break
			}
			apiChart(deps, nonce, chart, symbol, timespan, &jsonResponse)

		default:
			sublog.Error().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", endpoint).Err(fmt.Errorf("failure: call to unknown api endpoint")).Msg("api call failed")
			jsonResponse.Success = false
			jsonResponse.Message = "Failure: unknown endpoint"
		}

		json.NewEncoder(w).Encode(jsonResponse)
	})
}

func apiQuotes(deps *Dependencies, symbolStr string, jsonR *jsonResponseData) {
	sublog := deps.logger

	symbols := strings.Split(symbolStr, ",")

	validTickers := []Ticker{}
	validSymbols := []string{}
	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		ticker := Ticker{TickerSymbol: symbol}
		err := ticker.getBySymbol(deps)
		if err != nil {
			sublog.Error().Str("symbol", symbol).Msg("failed to find ticker")
			continue
		}
		validSymbols = append(validSymbols, symbol)
		validTickers = append(validTickers, ticker)

		_, lastCheckedSince, updatingNewsNow := getLastDoneInfo(deps, "ticker_news", ticker.TickerSymbol)
		jsonR.Data[symbol+":last_checked_since"] = lastCheckedSince
		jsonR.Data[symbol+":updating_news_now"] = updatingNewsNow
	}

	_, lastCheckedSince, updatingNewsNow := getLastDoneInfo(deps, "financial_news", "stockwatch")
	jsonR.Data["last_checked_since"] = lastCheckedSince
	jsonR.Data["updating_news_now"] = updatingNewsNow

	// if the market is open, lets get a live quote
	if isMarketOpen() {
		quotes, err := loadMultiTickerQuotes(deps, symbols)
		if err != nil {
			sublog.Error().Msg("failed to get live quotes")
			jsonR.Success = false
			jsonR.Message = "Failure: could not load quote"
			return
		}

		for _, symbol := range validSymbols {
			quote, ok := quotes[symbol]
			if !ok {
				continue
			}

			var dailyMove = "unchanged"
			if quote.QuoteChange > 0 {
				dailyMove = "up"
			} else if quote.QuoteChange < 0 {
				dailyMove = "down"
			}

			if quote.QuotePrice > 0 {
				jsonR.Data[symbol+":quote_shareprice"] = fmt.Sprintf("$%.2f", quote.QuotePrice)
				jsonR.Data[symbol+":quote_ask"] = fmt.Sprintf("$%.2f", quote.QuoteAsk)
				jsonR.Data[symbol+":quote_asksize"] = fmt.Sprintf("%d", quote.QuoteAskSize)
				jsonR.Data[symbol+":quote_bid"] = fmt.Sprintf("$%.2f", quote.QuoteBid)
				jsonR.Data[symbol+":quote_bidsize"] = fmt.Sprintf("%d", quote.QuoteBidSize)
				jsonR.Data[symbol+":quote_dailymove"] = dailyMove
				jsonR.Data[symbol+":quote_change"] = fmt.Sprintf("$%.2f", quote.QuoteChange)
				jsonR.Data[symbol+":quote_change_pct"] = fmt.Sprintf("%.2f%%", quote.QuoteChangePct)
				jsonR.Data[symbol+":quote_volume"] = fmt.Sprintf("%d", quote.QuoteVolume)
				jsonR.Data[symbol+":quote_asof"] = FormatUnixTime(quote.QuoteTime, "Jan 2 15:04:05")
				jsonR.Data[symbol+":quote_dailyrange"] = fmt.Sprintf("$%.2f - $%.2f", quote.QuoteLow, quote.QuoteHigh)
			}
		}
		jsonR.Data["is_market_open"] = true
		jsonR.Success = true
		jsonR.Message = "ok"
	} else {
		for x, symbol := range validSymbols {
			lastTickerDaily, err := getLastTickerDaily(deps, validTickers[x].TickerId)
			if err != nil {
				sublog.Error().Err(err).Str("symbol", symbol).Msg("failed to get last 2 dailys for {symbol}")
			}
			dailyMove, err := getLastTickerDailyMove(deps, validTickers[x].TickerId)
			if err != nil {
				sublog.Error().Err(err).Str("symbol", symbol).Msg("failed to get last 2 dailys for {symbol}")
			}

			jsonR.Data[symbol+":quote_shareprice"] = fmt.Sprintf("$%.2f", lastTickerDaily[0].ClosePrice)
			jsonR.Data[symbol+":quote_dailymove"] = dailyMove
			jsonR.Data[symbol+":quote_change"] = fmt.Sprintf("$%.2f", lastTickerDaily[0].ClosePrice-lastTickerDaily[1].ClosePrice)
			jsonR.Data[symbol+":quote_change_pct"] = fmt.Sprintf("%.2f%%", (lastTickerDaily[0].ClosePrice-lastTickerDaily[1].ClosePrice)/lastTickerDaily[1].ClosePrice*100)
			jsonR.Data[symbol+":quote_volume"] = fmt.Sprintf("%.0f", lastTickerDaily[0].Volume)
			jsonR.Data[symbol+":quote_asof"] = lastTickerDaily[0].PriceDatetime.Format("Jan 2")
			jsonR.Data[symbol+":quote_dailyrange"] = fmt.Sprintf("$%.2f - $%.2f", lastTickerDaily[0].LowPrice, lastTickerDaily[1].HighPrice)
		}

		jsonR.Data["is_market_open"] = false
		jsonR.Success = true
	}
}

func apiRecents(deps *Dependencies, watcher Watcher, action, symbolStr string, jsonR *jsonResponseData) {
	sublog := deps.logger

	symbols := strings.Split(symbolStr, ",")

	switch {
	case action == "remove":
		for _, symbol := range symbols {
			if symbol == "" {
				continue
			}
			ticker := Ticker{TickerSymbol: symbol}
			err := ticker.getBySymbol(deps)
			if err != nil {
				sublog.Error().Str("symbol", symbol).Msg("failed to find ticker")
				continue
			}
			removeFromWatcherRecents(deps, watcher, ticker)
		}
	case action == "lock":
		for _, symbol := range symbols {
			if symbol == "" {
				continue
			}
			ticker := Ticker{TickerSymbol: symbol}
			err := ticker.getBySymbol(deps)
			if err != nil {
				sublog.Error().Str("symbol", symbol).Msg("failed to find ticker")
				continue
			}
			lockWatcherRecent(deps, watcher, ticker)
		}
	case action == "unlock":
		for _, symbol := range symbols {
			if symbol == "" {
				continue
			}
			ticker := Ticker{TickerSymbol: symbol}
			err := ticker.getBySymbol(deps)
			if err != nil {
				sublog.Error().Str("symbol", symbol).Msg("failed to find ticker")
				continue
			}
			unlockWatcherRecent(deps, watcher, ticker)
		}
	}
	jsonR.Success = true
	jsonR.Message = "ok"
}

func apiChart(deps *Dependencies, nonce string, chart string, symbol string, timespan int, jsonR *jsonResponseData) {
	sublog := deps.logger.With().Str("chart", chart).Str("symbol", symbol).Int("timespan", timespan).Logger()

	start := time.Now()

	ticker := Ticker{TickerSymbol: symbol}
	err := ticker.getBySymbol(deps)
	if err != nil {
		sublog.Error().Msg("failed to find symbol {symbol}")
		jsonR.Success = false
		jsonR.Message = "Failure: unknown symbol"
		return
	}
	exchange := Exchange{ExchangeId: uint64(ticker.ExchangeId)}
	err = exchange.getById(deps)
	if err != nil {
		sublog.Error().Msg("failed to find exchange for {symbol}")
		jsonR.Success = false
		jsonR.Message = "Failure: unknown symbol"
		return
	}

	switch chart {
	case "symbolLine":
		ticker_dailies, _ := ticker.getTickerEODs(deps, timespan)
		webwatches, _ := loadWebWatches(deps, ticker.TickerId)
		chartHTML := chartHandlerTickerDailyLine(deps, ticker, &exchange, ticker_dailies, webwatches)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "symbolKline":
		ticker_dailies, _ := ticker.getTickerEODs(deps, timespan)
		webwatches, _ := loadWebWatches(deps, ticker.TickerId)
		chartHTML := chartHandlerTickerDailyKLine(deps, ticker, &exchange, ticker_dailies, webwatches)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialQuarterlyBar":
		qtrBarStrs, qtrBarValues, _ := ticker.GetFinancials(deps, "Quarterly", "bar", 0)
		chartHTML := chartHandlerFinancialsBar(deps, ticker, &exchange, qtrBarStrs, qtrBarValues)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialAnnualBar":
		annBarStrs, annBarValues, _ := ticker.GetFinancials(deps, "Annual", "bar", 0)
		chartHTML := chartHandlerFinancialsBar(deps, ticker, &exchange, annBarStrs, annBarValues)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialQuarterlyLine":
		qtrLineStrs, qtrLineValues, _ := ticker.GetFinancials(deps, "Quarterly", "line", 0)
		chartHTML := chartHandlerFinancialsLine(deps, ticker, &exchange, qtrLineStrs, qtrLineValues, 0)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialAnnualLine":
		annLineStrs, annLineValues, _ := ticker.GetFinancials(deps, "Annual", "line", 0)
		chartHTML := chartHandlerFinancialsLine(deps, ticker, &exchange, annLineStrs, annLineValues, 0)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialQuarterlyPerc":
		qtrPercStrs, qtrPercValues, _ := ticker.GetFinancials(deps, "Quarterly", "line", 1)
		chartHTML := chartHandlerFinancialsLine(deps, ticker, &exchange, qtrPercStrs, qtrPercValues, 1)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialAnnualcwPercLine":
		annPercStrs, annPercValues, _ := ticker.GetFinancials(deps, "Annual", "line", 1)
		chartHTML := chartHandlerFinancialsLine(deps, ticker, &exchange, annPercStrs, annPercValues, 1)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	default:
		sublog.Error().Msg("unknown chart type {chart_type}")
		jsonR.Success = false
		jsonR.Message = "Failure: unknown symbol"
	}

	sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Msg("timer: build chart")
}
