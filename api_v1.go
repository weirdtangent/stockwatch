package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type jsonResponseData struct {
	ApiVersion string                 `json:"api_version"`
	Endpoint   string                 `json:"endpoint"`
	Success    bool                   `json:"success"`
	Message    string                 `json:"message"`
	Data       map[string]interface{} `json:"data"`
}

// handles:
//   /api/version
//   /api/quotes
//   /api/recents
//   /api/chart

func apiV1Handler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		watcher := checkAuthState(w, r, deps, *deps.logger)

		w.Header().Add("Content-Type", "application/json")

		// get already inplace nonce from the current page and use it so our answer is allowed
		reqHeader := r.Header
		nonce := reqHeader.Get("X-Nonce")
		deps.nonce = nonce // we don't use webdata in api, so no need to fix that one

		params := mux.Vars(r)
		endpoint := params["endpoint"]

		jsonResponse := jsonResponseData{
			ApiVersion: "0.1.0",
			Endpoint:   endpoint,
			Success:    false,
			Data:       make(map[string]interface{}),
		}
		sublog := deps.logger.With().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", jsonResponse.Endpoint).Logger()

		switch endpoint {
		case "version":
			jsonResponse.Success = true
			jsonResponse.Message = "ok"

		case "quotes":
			symbolStr := r.FormValue("symbols")
			apiQuotes(deps, sublog, symbolStr, &jsonResponse)

		case "recents":
			if r.FormValue("remove") != "" {
				removeStr := r.FormValue("remove")
				apiRecents(deps, sublog, watcher, "remove", removeStr, &jsonResponse)
			} else if r.FormValue("lock") != "" {
				lockStr := r.FormValue("lock")
				apiRecents(deps, sublog, watcher, "lock", lockStr, &jsonResponse)
			} else if r.FormValue("unlock") != "" {
				unlockStr := r.FormValue("unlock")
				apiRecents(deps, sublog, watcher, "unlock", unlockStr, &jsonResponse)
			}

		case "chart":
			chart := r.FormValue("chart")
			symbol := r.FormValue("symbol")
			timespan, err := strconv.Atoi(r.FormValue("timespan"))
			if err != nil {
				jsonResponse.Success = false
				jsonResponse.Message = "failure: invalid timespan"
				break
			}
			apiChart(deps, sublog, nonce, chart, symbol, timespan, &jsonResponse)

		default:
			jsonResponse.Success = false
			jsonResponse.Message = "failure: unknown endpoint"
		}

		json.NewEncoder(w).Encode(jsonResponse)
	})
}

func apiQuotes(deps *Dependencies, sublog zerolog.Logger, symbolStr string, jsonR *jsonResponseData) {
	quotes, err := loadMultiTickerQuotes(deps, sublog, strings.Split(symbolStr, ","))
	if err != nil {
		sublog.Error().Msg("failed to get live quotes")
		jsonR.Success = false
		jsonR.Message = "failure: could not load quote"
		return
	}

	for _, quote := range quotes {
		symbol := quote.Symbol
		ticker, err := getTickerBySymbol(deps, sublog, symbol)
		if err != nil {
			sublog.Error().Str("symbol", symbol).Msg("failed to find ticker")
			jsonR.Success = false
			jsonR.Message = "failure: could not load quote"
			return
		}
		ticker.UpdateTickerWithLiveQuote(deps, sublog, quote)
		jsonR.Data[symbol+":price"] = fmt.Sprintf("$%.2f", ticker.MarketPrice)
		jsonR.Data[symbol+":ask"] = fmt.Sprintf("$%.2f", quote.QuoteAsk)
		jsonR.Data[symbol+":asksize"] = fmt.Sprintf("%d", quote.QuoteAskSize)
		jsonR.Data[symbol+":bid"] = fmt.Sprintf("$%.2f", quote.QuoteBid)
		jsonR.Data[symbol+":bidsize"] = fmt.Sprintf("%d", quote.QuoteBidSize)
		jsonR.Data[symbol+":change_amt"] = fmt.Sprintf("$%.2f", ticker.MarketPrice-ticker.MarketPrevClose)
		jsonR.Data[symbol+":change_pct"] = fmt.Sprintf("%.2f%%", (ticker.MarketPrice-ticker.MarketPrevClose)/ticker.MarketPrevClose*100)
		if ticker.MarketPrice-ticker.MarketPrevClose > 0 {
			jsonR.Data[symbol+":change_dir"] = "up"
		} else if ticker.MarketPrice-ticker.MarketPrevClose < 0 {
			jsonR.Data[symbol+":change_dir"] = "down"
		} else {
			jsonR.Data[symbol+":change_dir"] = "unchanged"
		}
		jsonR.Data[symbol+":volume"] = fmt.Sprintf("%d", ticker.MarketVolume)
		if isMarketOpen() {
			jsonR.Data[symbol+":asof"] = ticker.MarketPriceDatetime.Format("Jan 02 15:04:05")
		} else {
			jsonR.Data[symbol+":asof"] = ticker.MarketPriceDatetime.Format("Jan 02 15:04")
		}
		jsonR.Data[symbol+":dailyrange"] = fmt.Sprintf("$%.2f - $%.2f", quote.QuoteLow, quote.QuoteHigh)

		_, lastChecked, updatingNow := getLastDoneInfo(deps, sublog, "ticker_news", ticker.TickerSymbol)
		jsonR.Data[symbol+":last_checked"] = lastChecked
		jsonR.Data[symbol+":updating_now"] = updatingNow
	}

	_, lastCheckedSince, updatingNewsNow := getLastDoneInfo(deps, sublog, "financial_news", "stockwatch")
	jsonR.Data["last_checked_since"] = lastCheckedSince
	jsonR.Data["updating_news_now"] = updatingNewsNow

	jsonR.Data["is_market_open"] = isMarketOpen()
	jsonR.Success = true
}

func apiRecents(deps *Dependencies, sublog zerolog.Logger, watcher Watcher, action, symbolStr string, jsonR *jsonResponseData) {
	symbols := strings.Split(symbolStr, ",")

	switch {
	case action == "remove":
		for _, symbol := range symbols {
			if symbol == "" {
				continue
			}
			ticker, err := getTickerBySymbol(deps, sublog, symbol)
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
			ticker, err := getTickerBySymbol(deps, sublog, symbol)
			if err != nil {
				sublog.Error().Str("symbol", symbol).Msg("failed to find ticker")
				continue
			}
			lockWatcherRecent(deps, sublog, watcher, ticker)
		}
	case action == "unlock":
		for _, symbol := range symbols {
			if symbol == "" {
				continue
			}
			ticker, err := getTickerBySymbol(deps, sublog, symbol)
			if err != nil {
				sublog.Error().Str("symbol", symbol).Msg("failed to find ticker")
				continue
			}
			unlockWatcherRecent(deps, sublog, watcher, ticker)
		}
	}
	jsonR.Success = true
	jsonR.Message = "ok"
}

func apiChart(deps *Dependencies, sublog zerolog.Logger, nonce string, chart string, symbol string, timespan int, jsonR *jsonResponseData) {
	sublog = sublog.With().Str("chart", chart).Str("symbol", symbol).Int("timespan", timespan).Logger()

	start := time.Now()

	ticker, err := getTickerBySymbol(deps, sublog, symbol)
	if err != nil {
		sublog.Error().Msg("failed to find symbol {symbol}")
		jsonR.Success = false
		jsonR.Message = "failure: unknown symbol"
		return
	}
	exchange, err := getExchangeById(deps, sublog, ticker.ExchangeId)
	if err != nil {
		sublog.Error().Msg("failed to find exchange for {symbol}")
		jsonR.Success = false
		jsonR.Message = "failure: unknown symbol"
		return
	}

	switch chart {
	case "symbolLine":
		ticker_dailies, _ := ticker.getTickerEODs(deps, sublog, timespan)
		webwatches, _ := loadWebWatches(deps, sublog, ticker.TickerId)
		chartHTML := chartHandlerTickerDailyLine(deps, sublog, ticker, &exchange, ticker_dailies, webwatches)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "symbolKline":
		ticker_dailies, _ := ticker.getTickerEODs(deps, sublog, timespan)
		webwatches, _ := loadWebWatches(deps, sublog, ticker.TickerId)
		chartHTML := chartHandlerTickerDailyKLine(deps, sublog, ticker, &exchange, ticker_dailies, webwatches)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialQuarterlyBar":
		qtrBarStrs, qtrBarValues, _ := ticker.GetFinancials(deps, sublog, "Quarterly", "bar", 0)
		chartHTML := chartHandlerFinancialsBar(deps, sublog, ticker, &exchange, qtrBarStrs, qtrBarValues)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialAnnualBar":
		annBarStrs, annBarValues, _ := ticker.GetFinancials(deps, sublog, "Annual", "bar", 0)
		chartHTML := chartHandlerFinancialsBar(deps, sublog, ticker, &exchange, annBarStrs, annBarValues)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialQuarterlyLine":
		qtrLineStrs, qtrLineValues, _ := ticker.GetFinancials(deps, sublog, "Quarterly", "line", 0)
		chartHTML := chartHandlerFinancialsLine(deps, sublog, ticker, &exchange, qtrLineStrs, qtrLineValues, 0)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialAnnualLine":
		annLineStrs, annLineValues, _ := ticker.GetFinancials(deps, sublog, "Annual", "line", 0)
		chartHTML := chartHandlerFinancialsLine(deps, sublog, ticker, &exchange, annLineStrs, annLineValues, 0)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialQuarterlyPerc":
		qtrPercStrs, qtrPercValues, _ := ticker.GetFinancials(deps, sublog, "Quarterly", "line", 1)
		chartHTML := chartHandlerFinancialsLine(deps, sublog, ticker, &exchange, qtrPercStrs, qtrPercValues, 1)
		jsonR.Data["chartHTML"] = chartHTML
		jsonR.Success = true
		jsonR.Message = "ok"
	case "financialAnnualcwPercLine":
		annPercStrs, annPercValues, _ := ticker.GetFinancials(deps, sublog, "Annual", "line", 1)
		chartHTML := chartHandlerFinancialsLine(deps, sublog, ticker, &exchange, annPercStrs, annPercValues, 1)
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
