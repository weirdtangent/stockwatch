package main

import (
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type SearchResultNews struct {
	Publisher   string
	Title       string
	Type        string
	URL         string
	PublishDate string
}

type SearchResultTicker struct {
	TickerSymbol    string
	ExchangeAcronym string
	ShortName       string
	LongName        string
	Type            string
	SearchScore     int64
}

type SearchResult struct {
	ResultType string
	News       SearchResultNews
	Ticker     SearchResultTicker
}

func searchHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		awssess := ctx.Value("awssess").(*session.Session)
		webdata := ctx.Value("webdata").(map[string]interface{})
		messages := ctx.Value("messages").(*[]Message)

		params := mux.Vars(r)
		searchType := params["type"]

		if ok := checkAuthState(w, r); ok == false {
			encoded, err := encryptURL(awssess, ([]byte(r.URL.String())))
			if err == nil {
				http.Redirect(w, r, "/?next="+string(encoded), 302)
			} else {
				http.Redirect(w, r, "/", 302)
			}
			return
		}

		switch searchType {
		case "ticker":
			searchString := r.FormValue("searchString")
			searchType := r.FormValue("submit")
			if searchString == "" || searchType == "" {
				*messages = append(*messages, Message{fmt.Sprintf("Search text not entered or invalid search function"), "warning"})
				break
			}
			webdata["searchString"] = searchString

			logger.Info().
				Str("search_provider", "yahoofinance").
				Str("search_type", searchType).
				Str("search_string", searchString).
				Msg("Search performed")

			if searchType == "jump" {
				searchResult, err := jumpSearch(ctx, searchString)
				if err != nil {
					*messages = append(*messages, Message{fmt.Sprintf("Sorry, error returned for that search"), "danger"})
					break
				}
				if searchResult.Ticker.TickerSymbol == "" {
					*messages = append(*messages, Message{fmt.Sprintf("Sorry, nothing found for '%s'", searchString), "warning"})
					break
				}
				log.Info().
					Str("search_provider", "yahoofinance").
					Str("search_type", searchType).
					Str("search_string", searchString).
					Str("symbol", searchResult.Ticker.TickerSymbol).
					Msg("Search results")
				http.Redirect(w, r, fmt.Sprintf("/view/%s/%s", searchResult.Ticker.TickerSymbol, searchResult.Ticker.ExchangeAcronym), http.StatusFound)
				return
			} else if searchType == "search" {
				searchResults, err := listSearch(ctx, searchString, "both")
				if err != nil {
					*messages = append(*messages, Message{fmt.Sprintf("Sorry, error returned for that search"), "danger"})
					break
				}
				if len(searchResults) == 0 {
					*messages = append(*messages, Message{fmt.Sprintf("Sorry, nothing found for '%s'", searchString), "warning"})
					break
				}
				log.Info().
					Str("search_provider", "yahoofinance").
					Str("search_type", searchType).
					Str("search_string", searchString).
					Int("results_count", len(searchResults)).
					Msg("Search results")
				webdata["results"] = searchResults
				break
			} else {
				*messages = append(*messages, Message{fmt.Sprintf("Sorry, search type is unknown"), "warning"})
				break
			}

		default:
			logger.Warn().
				Str("search_type", searchType).
				Msg("Unknown search_type")
			*messages = append(*messages, Message{fmt.Sprintf("Sorry, invalid search request"), "danger"})
		}

		renderTemplateDefault(w, r, "searchresults")
	})
}
