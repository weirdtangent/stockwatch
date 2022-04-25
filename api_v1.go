package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type jsonResponseData struct {
	ApiVersion string                 `json:"api_version"`
	Success    bool                   `json:"success"`
	Message    string                 `json:"message"`
	Data       map[string]interface{} `json:"data"`
}

func apiV1Handler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		checkAuthState(w, r)
		// if ok := checkAuthState(w, r); !ok {
		// 	http.NotFound(w, r)
		// 	return
		// }

		w.Header().Add("Content-Type", "application/json")

		var jsonResponse jsonResponseData
		jsonResponse.ApiVersion = "0.1.0"
		jsonResponse.Success = false
		jsonResponse.Data = make(map[string]interface{})

		params := mux.Vars(r)
		endpoint := params["endpoint"]

		//logger.Info().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", endpoint).Msg("Preparing to answer api request")

		switch endpoint {
		case "version":
			jsonResponse.Success = true
			jsonResponse.Message = "ok"

		case "quote":
			symbol := r.FormValue("symbol")

			ticker := Ticker{TickerSymbol: symbol}
			err := ticker.getBySymbol(ctx)
			if err != nil {
				zerolog.Ctx(ctx).Error().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", endpoint).Str("ticker", symbol).Msg("Failed to find ticker")
				jsonResponse.Success = false
				jsonResponse.Message = "Failure: unknown symbol"
			} else {
				// if the market is open, lets get a live quote
				if isMarketOpen() {
					quote, err := loadTickerQuote(ctx, ticker.TickerSymbol)

					if err != nil {
						zerolog.Ctx(ctx).Error().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", endpoint).Str("ticker", symbol).Msg("Failed to get live quote")
						jsonResponse.Success = false
						jsonResponse.Message = "Failure: could not load quote"
					} else {
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
							jsonResponse.Data["is_market_open"] = isMarketOpen()
						}
					}
				} else {
					jsonResponse.Success = true
					jsonResponse.Message = "Market closed, we already have latest info"
				}
			}

		default:
			zerolog.Ctx(ctx).Error().Str("api_version", jsonResponse.ApiVersion).Str("endpoint", endpoint).Err(fmt.Errorf("failure: call to unknown api endpoint")).Msg("")
			jsonResponse.Success = false
			jsonResponse.Message = "Failure: unknown endpoint"
		}

		json.NewEncoder(w).Encode(jsonResponse)
	})
}
