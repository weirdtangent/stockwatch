package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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
		ctx := r.Context()

		checkAuthState(w, r)

		w.Header().Add("Content-Type", "application/json")

		params := mux.Vars(r)
		endpoint := params["endpoint"]

		jsonResponse := jsonResponseData{ApiVersion: "0.1.0", Endpoint: endpoint, Success: false, Data: make(map[string]interface{})}

		switch endpoint {
		case "version":
			jsonResponse.Success = true
			jsonResponse.Message = "ok"

		case "quote":
			apiQuote(r, &jsonResponse)

		case "quotes":
			apiQuotes(r, &jsonResponse)

		default:
			zerolog.Ctx(ctx).Error().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", endpoint).Err(fmt.Errorf("failure: call to unknown api endpoint")).Msg("api call failed")
			jsonResponse.Success = false
			jsonResponse.Message = "Failure: unknown endpoint"
		}

		json.NewEncoder(w).Encode(jsonResponse)
	})
}

func apiQuote(r *http.Request, jsonResponse *jsonResponseData) {
	ctx := r.Context()
	symbol := r.FormValue("symbol")

	zerolog.Ctx(ctx).With().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", jsonResponse.Endpoint).Str("symbol", symbol)

	ticker := Ticker{TickerSymbol: symbol}
	err := ticker.getBySymbol(ctx)
	if err != nil {
		zerolog.Ctx(ctx).Error().Msg("Failed to find ticker")
		jsonResponse.Success = false
		jsonResponse.Message = "Failure: unknown symbol"
		return
	}

	newsLastUpdated, updatingNewsNow := getNewsLastUpdated(ctx, ticker)
	if newsLastUpdated.Valid {
		jsonResponse.Data["news_last_updated"] = newsLastUpdated.Time.Format("Jan 02 15:04")
	} else {
		if updatingNewsNow {
			jsonResponse.Data["news_last_updated"] = "checking now"
		} else {
			jsonResponse.Data["news_last_updated"] = "-"
		}
	}
	jsonResponse.Data["updating_news_now"] = updatingNewsNow

	// if the market is open, lets get a live quote
	if isMarketOpen() {
		quote, err := loadTickerQuote(ctx, ticker.TickerSymbol)
		if err != nil {
			zerolog.Ctx(ctx).Error().Msg("Failed to get live quote")
			jsonResponse.Success = false
			jsonResponse.Message = "Failure: could not load quote"
			return
		}
		jsonResponse.Success = true
		jsonResponse.Message = "ok"

		var dailyMove = "unchanged"
		if quote.QuoteChange > 0 {
			dailyMove = "up"
		} else if quote.QuoteChange < 0 {
			dailyMove = "down"
		}

		if quote.QuotePrice > 0 {
			jsonResponse.Data["quote_shareprice"] = fmt.Sprintf("$%.2f", quote.QuotePrice)
			jsonResponse.Data["quote_ask"] = fmt.Sprintf("$%.2f", quote.QuoteAsk)
			jsonResponse.Data["quote_asksize"] = quote.QuoteAskSize
			jsonResponse.Data["quote_bid"] = fmt.Sprintf("$%.2f", quote.QuoteBid)
			jsonResponse.Data["quote_bidsize"] = quote.QuoteBidSize
			jsonResponse.Data["quote_dailymove"] = dailyMove
			jsonResponse.Data["quote_change"] = fmt.Sprintf("$%.2f", quote.QuoteChange)
			jsonResponse.Data["quote_change_pct"] = fmt.Sprintf("%.2f%%", quote.QuoteChangePct)
			jsonResponse.Data["quote_volume"] = quote.QuoteVolume
			jsonResponse.Data["quote_asof"] = FormatUnixTime(quote.QuoteTime, "Jan 2 15:04")
			jsonResponse.Data["quote_dailyrange"] = fmt.Sprintf("$%.2f - $%.2f", quote.QuoteLow, quote.QuoteHigh)
			jsonResponse.Data["is_market_open"] = true
		}
	} else {
		jsonResponse.Data["is_market_open"] = false
		jsonResponse.Success = true
		jsonResponse.Message = "Market closed, we already have latest info"
	}
}

func apiQuotes(r *http.Request, jsonResponse *jsonResponseData) {
	ctx := r.Context()
	symbolStr := r.FormValue("symbols")
	symbols := strings.Split(symbolStr, ",")

	zerolog.Ctx(ctx).With().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", jsonResponse.Endpoint).Str("symbols", symbolStr)

	validSymbols := []string{}
	for _, symbol := range symbols {
		ticker := Ticker{TickerSymbol: symbol}
		err := ticker.getBySymbol(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Error().Msg("Failed to find ticker")
			continue
		}

		validSymbols = append(validSymbols, symbol)
		newsLastUpdated, updatingNewsNow := getNewsLastUpdated(ctx, ticker)
		if newsLastUpdated.Valid {
			jsonResponse.Data[symbol+"|news_last_updated"] = newsLastUpdated.Time.Format("Jan 02 15:04")
		} else {
			if updatingNewsNow {
				jsonResponse.Data[symbol+"|news_last_updated"] = "checking now"
			} else {
				jsonResponse.Data[symbol+"|news_last_updated"] = "-"
			}
		}
		jsonResponse.Data[symbol+"updating_news_now"] = updatingNewsNow
	}

	// if the market is open, lets get a live quote
	if isMarketOpen() {
		quotes, err := loadMultiTickerQuotes(ctx, symbols)
		if err != nil {
			zerolog.Ctx(ctx).Error().Msg("Failed to get live quotes")
			jsonResponse.Success = false
			jsonResponse.Message = "Failure: could not load quote"
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
				jsonResponse.Data[symbol+"|quote_shareprice"] = fmt.Sprintf("$%.2f", quote.QuotePrice)
				jsonResponse.Data[symbol+"|quote_ask"] = fmt.Sprintf("$%.2f", quote.QuoteAsk)
				jsonResponse.Data[symbol+"|quote_asksize"] = quote.QuoteAskSize
				jsonResponse.Data[symbol+"|quote_bid"] = fmt.Sprintf("$%.2f", quote.QuoteBid)
				jsonResponse.Data[symbol+"|quote_bidsize"] = quote.QuoteBidSize
				jsonResponse.Data[symbol+"|quote_dailymove"] = dailyMove
				jsonResponse.Data[symbol+"|quote_change"] = fmt.Sprintf("$%.2f", quote.QuoteChange)
				jsonResponse.Data[symbol+"|quote_change_pct"] = fmt.Sprintf("%.2f%%", quote.QuoteChangePct)
				jsonResponse.Data[symbol+"|quote_volume"] = quote.QuoteVolume
				jsonResponse.Data[symbol+"|quote_asof"] = FormatUnixTime(quote.QuoteTime, "Jan 2 15:04")
				jsonResponse.Data[symbol+"|quote_dailyrange"] = fmt.Sprintf("$%.2f - $%.2f", quote.QuoteLow, quote.QuoteHigh)
			}
		}
		jsonResponse.Data["is_market_open"] = true
		jsonResponse.Success = true
		jsonResponse.Message = "ok"
	} else {
		jsonResponse.Data["is_market_open"] = false
		jsonResponse.Success = true
		jsonResponse.Message = "Market closed, we already have latest info"
	}
}
