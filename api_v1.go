package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

func apiV1Handler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := checkAuthState(w, r)

		w.Header().Add("Content-Type", "application/json")

		params := mux.Vars(r)
		endpoint := params["endpoint"]

		jsonResponse := jsonResponseData{ApiVersion: "0.1.0", Endpoint: endpoint, Success: false, Data: make(map[string]interface{})}
		log := zerolog.Ctx(ctx).With().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", jsonResponse.Endpoint).Logger()

		switch endpoint {
		case "version":
			jsonResponse.Success = true
			jsonResponse.Message = "ok"

		case "quotes":
			symbolStr := r.FormValue("symbols")
			log = log.With().Str("symbols", symbolStr).Logger()
			ctx = log.WithContext(ctx)
			apiQuotes(ctx, symbolStr, &jsonResponse)

		default:
			zerolog.Ctx(ctx).Error().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", endpoint).Err(fmt.Errorf("failure: call to unknown api endpoint")).Msg("api call failed")
			jsonResponse.Success = false
			jsonResponse.Message = "Failure: unknown endpoint"
		}

		json.NewEncoder(w).Encode(jsonResponse)
	})
}

func apiQuotes(ctx context.Context, symbolStr string, jsonR *jsonResponseData) {
	symbols := strings.Split(symbolStr, ",")

	validTickers := []Ticker{}
	validSymbols := []string{}
	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		ticker := Ticker{TickerSymbol: symbol}
		err := ticker.getBySymbol(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Error().Str("symbol", symbol).Msg("Failed to find ticker")
			continue
		}
		validSymbols = append(validSymbols, symbol)
		validTickers = append(validTickers, ticker)

		newsLastUpdated, updatingNewsNow := getNewsLastUpdated(ctx, ticker)
		if newsLastUpdated.Valid {
			jsonR.Data[symbol+":last_checked_news"] = newsLastUpdated.Time.Format("Jan 02 15:04")
		} else {
			if updatingNewsNow {
				jsonR.Data[symbol+":last_checked_news"] = "checking now"
			} else {
				jsonR.Data[symbol+":last_checked_news"] = "not yet"
			}
		}
		jsonR.Data[symbol+":updating_news_now"] = updatingNewsNow
	}

	// if the market is open, lets get a live quote
	if isMarketOpen() {
		quotes, err := loadMultiTickerQuotes(ctx, symbols)
		if err != nil {
			zerolog.Ctx(ctx).Error().Msg("Failed to get live quotes")
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
		jsonR.Data["is_market_open"] = strconv.FormatBool(true)
		jsonR.Success = true
		jsonR.Message = "ok"
	} else {
		for x, symbol := range validSymbols {
			lastTickerDaily, err := getLastTickerDaily(ctx, validTickers[x].TickerId)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("symbol", symbol).Msg("failed to get last 2 dailys for {symbol}")
			}
			dailyMove, err := getLastTickerDailyMove(ctx, validTickers[x].TickerId)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("symbol", symbol).Msg("failed to get last 2 dailys for {symbol}")
			}

			jsonR.Data[symbol+":quote_shareprice"] = fmt.Sprintf("$%.2f", lastTickerDaily[0].ClosePrice)
			jsonR.Data[symbol+":quote_dailymove"] = dailyMove
			jsonR.Data[symbol+":quote_change"] = fmt.Sprintf("$%.2f", lastTickerDaily[0].ClosePrice-lastTickerDaily[1].ClosePrice)
			jsonR.Data[symbol+":quote_change_pct"] = fmt.Sprintf("%.2f%%", (lastTickerDaily[0].ClosePrice-lastTickerDaily[1].ClosePrice)/lastTickerDaily[1].ClosePrice*100)
			jsonR.Data[symbol+":quote_volume"] = fmt.Sprintf("%.0f", lastTickerDaily[0].Volume)
			jsonR.Data[symbol+":quote_asof"] = lastTickerDaily[0].PriceDatetime.Format("Jan 2")
			jsonR.Data[symbol+":quote_dailyrange"] = fmt.Sprintf("$%.2f - $%.2f", lastTickerDaily[0].LowPrice, lastTickerDaily[1].HighPrice)
		}

		jsonR.Data["is_market_open"] = strconv.FormatBool(false)
		jsonR.Success = true
	}
}
