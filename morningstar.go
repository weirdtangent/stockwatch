package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/morningstar"
)

// load movers (gainers, losers, and actives)
func loadMovers(ctx context.Context) error {
	logger := log.Ctx(ctx)

	apiKey := ctx.Value("morningstar_apikey").(string)
	apiHost := ctx.Value("morningstar_apihost").(string)

	sourceId, err := getSourceId("Morningstar")
	if err != nil {
		log.Error().Err(err).Msg("Failed to find sourceId for API source")
		return err
	}

	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	currentDateStr := currentDate.Format("2006-01-02")

	moversParams := map[string]string{}
	response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "movers", moversParams)
	if err != nil {
		logger.Warn().Err(err).
			Msg("Failed to retrieve movers")
		return err
	}

	var moversResponse morningstar.MSMoversResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&moversResponse)

	for _, gainer := range moversResponse.Gainers {
		var err error

		ticker, err := getTickerBySymbol(ctx, gainer.Symbol)
		if err != nil {
			time.Sleep(5 * time.Second) // wait 5 seconds between slamming yahoofinance with new ticker requests
			ticker, err = loadTicker(ctx, gainer.Symbol)
			if err != nil {
				logger.Error().Err(err).Str("ticker", gainer.Symbol).Msg("Failed to find or load gainer ticker")
			}
		}
		if err == nil {
			mover := Mover{0, sourceId, ticker.TickerId, currentDateStr, "gainer", gainer.LastPrice, gainer.PriceChange, gainer.PriceChangePct, gainer.Volume, "", ""}
			mover.createIfNew(ctx)
		}
	}

	for _, loser := range moversResponse.Losers {
		ticker, err := getTickerBySymbol(ctx, loser.Symbol)
		if err != nil {
			time.Sleep(5 * time.Second) // wait 5 seconds between slamming yahoofinance with new ticker requests
			ticker, err = loadTicker(ctx, loser.Symbol)
			if err != nil {
				logger.Error().Err(err).Str("ticker", loser.Symbol).Msg("Failed to find or load loser ticker")
			}
		}
		if err == nil {
			mover := Mover{0, sourceId, ticker.TickerId, currentDateStr, "loser", loser.LastPrice, loser.PriceChange, loser.PriceChangePct, loser.Volume, "", ""}
			mover.createIfNew(ctx)
		}
	}

	for _, active := range moversResponse.Actives {
		ticker, err := getTickerBySymbol(ctx, active.Symbol)
		if err != nil {
			time.Sleep(5 * time.Second) // wait 5 seconds between slamming yahoofinance with new ticker requests
			ticker, err = loadTicker(ctx, active.Symbol)
			if err != nil {
				logger.Error().Err(err).Str("ticker", active.Symbol).Msg("Failed to find or load active ticker")
			}
		}
		if err == nil {
			mover := Mover{0, sourceId, ticker.TickerId, currentDateStr, "active", active.LastPrice, active.PriceChange, active.PriceChangePct, active.Volume, "", ""}
			mover.createIfNew(ctx)
		}
	}
	return nil
}

// load news
func loadMSNewsArticles(ctx context.Context, query string) error {
	logger := log.Ctx(ctx)

	apiKey := ctx.Value("morningstar_apikey").(string)
	apiHost := ctx.Value("morningstar_apihost").(string)

	performanceIds := make(map[string]bool)

	sourceId, err := getSourceId("Morningstar")
	if err != nil {
		log.Error().Err(err).Msg("Failed to find sourceId for API source")
		return err
	}

	autoCompleteParams := map[string]string{}
	autoCompleteParams["q"] = query
	response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "autocomplete", autoCompleteParams)
	if err != nil {
		logger.Warn().Err(err).
			Msg("Failed to retrieve autocomplete")
		return err
	}

	var autoCompleteResponse morningstar.MSAutoCompleteResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&autoCompleteResponse)

	for _, result := range autoCompleteResponse.Results {
		performanceId := result.PerformanceId
		if _, ok := performanceIds[performanceId]; ok == false {
			performanceIds[performanceId] = true

			articleListParams := map[string]string{}
			articleListParams["performanceId"] = performanceId
			logger.Info().Str("performance_id", performanceId).Msg("Checking for news articles for performance_id")
			response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "articlelist", articleListParams)
			if err != nil {
				logger.Warn().Err(err).Str("performanceId", performanceId).
					Msg("Failed to retrieve articleList")
				return err
			}

			var articlesListResponse []morningstar.MSArticlesListResponse
			json.NewDecoder(strings.NewReader(response)).Decode(&articlesListResponse)

			for _, article := range articlesListResponse {
				if existingId, err := getArticleByExternalId(ctx, sourceId, article.InternalId); err == nil && existingId == 0 {
				}
			}
		}
	}
	return nil
}
