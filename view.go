package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/weirdtangent/mymath"
)

func viewTickerDailyHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		messages := ctx.Value(ContextKey("messages")).(*[]Message)
		webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})

		checkAuthState(w, r)

		// if ctx, ok := checkAuthState(w, r); !ok {
		// 	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		// 	return
		// }

		params := mux.Vars(r)
		symbol := params["symbol"]

		timespan := 180
		if tsParam := r.FormValue("ts"); tsParam != "" {
			if tsValue, err := strconv.ParseInt(tsParam, 10, 32); err == nil {
				timespan = int(mymath.MinMax(tsValue, 15, 1825))
			} else if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Str("ts", tsParam).Msg("invalid timespan (ts) param")
			}
			zerolog.Ctx(ctx).Info().Int("timespan", timespan).Msg("")
		}

		ticker, err := loadTickerDetails(ctx, symbol, timespan)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("Failed to load ticker details for viewing")
			*messages = append(*messages, Message{fmt.Sprintf("Sorry, I had trouble loading that stock: %s", err.Error()), "danger"})
			renderTemplateDefault(w, r, "desktop")
			return
		}

		// Add this ticker to recents list
		recents, err := addTickerToRecents(ctx, r, ticker)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("failed to add ticker to recents list")
		}
		webdata["recents"] = *recents

		renderTemplateDefault(w, r, "view-daily")
	})
}
