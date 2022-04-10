package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/morningstar"
)

// load movers (gainers, losers, and actives)
func loadMovers(ctx context.Context) error {
	logger := log.Ctx(ctx)

	apiKey := ctx.Value(ContextKey("morningstar_apikey")).(string)
	apiHost := ctx.Value(ContextKey("morningstar_apihost")).(string)

	sourceId, err := getSourceId("Morningstar")
	if err != nil {
		log.Error().Err(err).Msg("failed to find sourceId for API source")
		return err
	}

	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	currentDateStr := currentDate.Format("2006-01-02")

	moversParams := map[string]string{}
	response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "movers", moversParams)
	if err != nil {
		logger.Warn().Err(err).
			Msg("failed to retrieve movers")
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
				logger.Error().Err(err).Str("ticker", gainer.Symbol).Msg("failed to find or load gainer ticker")
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
				logger.Error().Err(err).Str("ticker", loser.Symbol).Msg("failed to find or load loser ticker")
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
				logger.Error().Err(err).Str("ticker", active.Symbol).Msg("failed to find or load active ticker")
			}
		}
		if err == nil {
			mover := Mover{0, sourceId, ticker.TickerId, currentDateStr, "active", active.LastPrice, active.PriceChange, active.PriceChangePct, active.Volume, "", ""}
			mover.createIfNew(ctx)
		}
	}
	return nil
}

// load article
// func loadMSNewsArticles(ctx context.Context, query string) error {
// 	logger := log.Ctx(ctx)

// 	apiKey := ctx.Value(ContextKey("morningstar_apikey")).(string)
// 	apiHost := ctx.Value(ContextKey("morningstar_apihost")).(string)

// 	performanceIds := make(map[string]bool)

// 	sourceId, err := getSourceId("Morningstar")
// 	if err != nil {
// 		log.Error().Err(err).Msg("failed to find sourceId for API source")
// 		return err
// 	}

// 	autoCompleteParams := map[string]string{}
// 	autoCompleteParams["q"] = query
// 	response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "autocomplete", autoCompleteParams)
// 	if err != nil {
// 		logger.Warn().Err(err).
// 			Msg("failed to retrieve autocomplete")
// 		return err
// 	}

// 	var autoCompleteResponse morningstar.MSAutoCompleteResponse
// 	json.NewDecoder(strings.NewReader(response)).Decode(&autoCompleteResponse)

// 	for _, result := range autoCompleteResponse.Results {
// 		performanceId := result.PerformanceId
// 		if _, ok := performanceIds[performanceId]; !ok {
// 			performanceIds[performanceId] = true

// 			articleListParams := map[string]string{}
// 			articleListParams["performanceId"] = performanceId
// 			logger.Info().Str("performance_id", performanceId).Msg("Checking for news articles for performance_id")
// 			response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "articlelist", articleListParams)
// 			if err != nil {
// 				logger.Warn().Err(err).Str("performanceId", performanceId).
// 					Msg("failed to retrieve articleList")
// 			} else {

// 				var articlesListResponse []morningstar.MSArticlesListResponse
// 				json.NewDecoder(strings.NewReader(response)).Decode(&articlesListResponse)

// 				for _, story := range articlesListResponse {
// 					internalId := fmt.Sprintf("%d", story.InternalId)
// 					if story.Status != "Live" {
// 						log.Info().Str("status", story.Status).Msg("skipped article because of status")
// 					} else {
// 						if existingId, err := getArticleByExternalId(ctx, sourceId, internalId); err != nil || existingId != 0 {
// 							log.Info().Err(err).Str("existing_id", internalId).Msg("skipped article because of err or we already have")
// 						} else {
// 							var publishedDate string
// 							var updatedAtDate string
// 							if published, err := strconv.ParseInt(story.Published[0:10], 10, 64); err == nil && published > 0 {
// 								publishedDate = FormatUnixTime(published, "2006-01-02 15:04:05")
// 							} else {
// 								log.Fatal().Err(err).Str("published", story.Published).Msg("failed to convert date")
// 							}
// 							if updatedat, err := strconv.ParseInt(story.UpdatedAt[0:10], 10, 64); err == nil && updatedat > 0 {
// 								updatedAtDate = FormatUnixTime(updatedat, "2006-01-02 15:04:05")
// 							} else {
// 								log.Fatal().Err(err).Str("updatedat", story.UpdatedAt).Msg("failed to convert date")
// 							}

// 							article := Article{0, sourceId, internalId, publishedDate, updatedAtDate, story.Title, story.Content.Body, story.Content.VideoFileURL, "", "", ""}

// 							err := article.createArticle(ctx)
// 							if err != nil {
// 								logger.Warn().Err(err).Str("id", query).Msg("failed to write new news article")
// 							}
// 							if article.ArticleId > 0 {
// 								for _, author := range story.Authors {
// 									if author.IsPrimary {
// 										for _, profile := range author.Profiles {
// 											if profile.IsPrimary {
// 												articleAuthor := ArticleAuthor{0, article.ArticleId, profile.ByLine, profile.JobTitle, profile.ShortBio, profile.LongBio, author.ImageURL, "", ""}
// 												err := articleAuthor.createArticleAuthor(ctx)
// 												if err != nil {
// 													logger.Warn().Err(err).Str("id", query).Msg("failed to write author(s) for new article")
// 												}
// 											}
// 										}
// 									}
// 								}

// 								for _, security := range story.Securities {
// 									ticker, err := getTickerBySymbol(ctx, security.Symbol)
// 									if err == nil {
// 										articleTicker := ArticleTicker{0, article.ArticleId, ticker.TickerSymbol, ticker.TickerId, "", ""}
// 										err := articleTicker.createArticleTicker(ctx)
// 										if err != nil {
// 											logger.Warn().Err(err).Str("id", query).Msg("failed to write ticker(s) for new article")
// 										}
// 									}
// 								}

// 								for _, keyword := range story.Keywords {
// 									articleKeyword := ArticleKeyword{0, article.ArticleId, keyword.Name, "", ""}
// 									err := articleKeyword.createArticleKeyword(ctx)
// 									if err != nil {
// 										logger.Warn().Err(err).Str("id", query).Msg("failed to write keyword(s) for new article")
// 									}
// 								}

// 								for _, tag := range story.Tags {
// 									articleTag := ArticleTag{0, article.ArticleId, tag.Name, "", ""}
// 									err := articleTag.createArticleTag(ctx)
// 									if err != nil {
// 										logger.Warn().Err(err).Str("id", query).Msg("failed to write tag(s) for new article")
// 									}
// 								}

// 							}
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

// load news
func loadMSNews(ctx context.Context, query string, ticker_id int64) error {
	logger := log.Ctx(ctx)

	apiKey := ctx.Value(ContextKey("morningstar_apikey")).(string)
	apiHost := ctx.Value(ContextKey("morningstar_apihost")).(string)

	performanceIds := make(map[string]bool)

	autoCompleteParams := map[string]string{}
	autoCompleteParams["q"] = query
	response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "autocomplete", autoCompleteParams)
	if err != nil {
		logger.Warn().Err(err).
			Msg("failed to retrieve autocomplete")
		return err
	}

	var autoCompleteResponse morningstar.MSAutoCompleteResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&autoCompleteResponse)

	for _, result := range autoCompleteResponse.Results {
		performanceId := result.PerformanceId
		if _, ok := performanceIds[performanceId]; !ok {
			performanceIds[performanceId] = true

			newsListParams := map[string]string{}
			newsListParams["performanceId"] = performanceId
			logger.Info().Str("performance_id", performanceId).Msg("Checking for news for performance_id")
			response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "newslist", newsListParams)
			if err != nil {
				logger.Warn().Err(err).Str("performanceId", performanceId).
					Msg("failed to retrieve newsList")
			} else {
				var newsListResponse []morningstar.MSNewsListResponse
				json.NewDecoder(strings.NewReader(response)).Decode(&newsListResponse)

				for _, story := range newsListResponse {
					sourceId, err := getSourceId(story.SourceId)
					if err != nil {
						log.Error().Err(err).Msg("failed story")
						continue
					}

					if existingId, err := getArticleByExternalId(ctx, sourceId, story.InternalId); err != nil || existingId != 0 {
						log.Info().Err(err).Str("existing_id", story.InternalId).Msg("skipped article because of err or we already have")
					} else {
						content, err := getNewsItemContent(ctx, story.SourceId, story.InternalId)
						if err != nil || len(content) == 0 {
							log.Error().Err(err).Msg("no news item content found")
							continue
						}

						publishedDateTime, err := time.Parse("2006-01-02T15:04:05-07:00", story.Published)
						if err != nil {
							log.Error().Err(err).Msg("could not parse Published")
							continue
						}
						publishedDate := publishedDateTime.Format("2006-01-02 15:04:05")

						article := Article{0, sourceId, story.InternalId, publishedDate, publishedDate, story.Title, content, "", "", "", ""}

						err = article.createArticle(ctx)
						if err != nil {
							logger.Warn().Err(err).Str("id", query).Msg("failed to write new news article")
						}

						articleTicker := ArticleTicker{0, article.ArticleId, query, ticker_id, "", ""}
						err = articleTicker.createArticleTicker(ctx)
						if err != nil {
							logger.Warn().Err(err).Str("id", query).Msg("failed to write ticker(s) for new article")
						}
					}
				}
			}
		}
	}
	return nil
}

// load news
func getNewsItemContent(ctx context.Context, sourceId string, internalId string) (string, error) {
	logger := log.Ctx(ctx)

	apiKey := ctx.Value(ContextKey("morningstar_apikey")).(string)
	apiHost := ctx.Value(ContextKey("morningstar_apihost")).(string)

	newsDetailsParams := map[string]string{}
	newsDetailsParams["id"] = internalId
	newsDetailsParams["sourceId"] = sourceId
	response, err := morningstar.GetFromMorningstar(&apiKey, &apiHost, "newsdetails", newsDetailsParams)
	if err != nil {
		logger.Warn().Err(err).
			Msg(fmt.Sprintf("failed to retrieve newsdetails for id/source %s/%s", internalId, sourceId))
		return "", err
	}

	var newsDetailsResponse morningstar.MSNewsDetailsResponse
	json.NewDecoder(strings.NewReader(response)).Decode(&newsDetailsResponse)

	newsContent := followContent(newsDetailsResponse.ContentObj)
	logger.Info().Msg(fmt.Sprintf("found content of %d bytes for article", len(newsContent)))
	return newsContent, nil
}

func followContent(contentObj []morningstar.MSNewsContentObj) string {
	noSpaces := regexp.MustCompile(`^\S+$`)

	var content string
	for _, contentPiece := range contentObj {
		var deeperContent string
		if len(contentPiece.ContentObj) > 0 {
			deeperContent = followContent(contentPiece.ContentObj)
		} else {
			deeperContent = contentPiece.Content
		}
		switch contentPiece.Type {
		case "text":
			content += deeperContent
		case "img":
			content += `<img src="` + contentPiece.Src + `">`
		case "a":
			if noSpaces.MatchString(deeperContent) {
				content += `<a href="` + deeperContent + `">` + deeperContent + `</a>`
			} else {
				content += deeperContent
			}
		default:
			content += `<` + contentPiece.Type + `>` + deeperContent + `</` + contentPiece.Type + `>`
		}
	}

	return content
}
