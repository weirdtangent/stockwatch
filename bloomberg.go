package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/bloomberg"
)

// load news
func loadBBNewsArticles(ctx context.Context, query string) error {
	logger := log.Ctx(ctx)

	apiKey := ctx.Value(ContextKey("bloomberg_apikey")).(string)
	apiHost := ctx.Value(ContextKey("bloomberg_apihost")).(string)

	sourceId, err := getSourceId("Bloomberg")
	if err != nil {
		log.Error().Err(err).Msg("Failed to find sourceId for API source")
		return err
	}

	// one of : markets|technology|view|pursuits|politics|green|citylab|businessweek|fixed-income|hyperdrive|cryptocurrencies|wealth|latest|personalFinance|quickTake|world|industries|stocks|currencies|brexit
	newsListParams := map[string]string{}
	newsListParams["id"] = query
	logger.Info().Str("id", query).Msg("Checking for news articles")
	response, err := bloomberg.GetFromBloombergMarket(&apiKey, &apiHost, "news", newsListParams)
	if err != nil {
		logger.Warn().Err(err).Str("id", query).
			Msg("Failed to retrieve newsList")
		return err
	}

	var newsListResponse bloomberg.BBNewsListResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&newsListResponse)

	for _, story := range newsListResponse.Modules[0].Stories {
		if existingId, err := getArticleByExternalId(ctx, sourceId, story.InternalId); err == nil && existingId == 0 {
			article := Article{0, sourceId, story.InternalId, FormatUnixTime(story.Published, "2006-01-02 15:04:05"), FormatUnixTime(story.UpdatedAt, "2006-01-02 15:04:05"), story.Title, "", story.LongURL, story.ThumbnailImage, "", ""}
			err := article.createArticle(ctx)
			if err != nil {
				logger.Warn().Err(err).Str("id", query).
					Msg("Failed to write new news article")
				return err
			}
			if article.ArticleId > 0 {
				articleAuthor := ArticleAuthor{0, article.ArticleId, story.ByLine, "", "", "", "", "", ""}
				err := articleAuthor.createArticleAuthor(ctx)
				if err != nil {
					logger.Warn().Err(err).Str("id", query).
						Msg("Failed to write author(s) for new article")
					return err
				}
			}
		}
	}
	return nil
}
