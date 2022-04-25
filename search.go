package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
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
	TickerSymbol string
	ExchangeMic  string
	ShortName    string
	LongName     string
	Type         string
	SearchScore  float64
}

type SearchResult struct {
	ResultType string
	News       SearchResultNews
	Ticker     SearchResultTicker
}

func searchHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})
		messages := ctx.Value(ContextKey("messages")).(*[]Message)

		checkAuthState(w, r)

		// if ctx, ok := checkAuthState(w, r); !ok {
		// 	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		// 	return
		// }

		params := mux.Vars(r)
		searchType := params["type"]

		switch searchType {
		case "ticker":
			searchString := r.FormValue("searchString")
			searchType := r.FormValue("submit")
			if searchString == "" || searchType == "" {
				*messages = append(*messages, Message{"search text not entered or invalid search function", "warning"})
				break
			}
			webdata["searchString"] = searchString

			zerolog.Ctx(ctx).Info().Str("search_provider", "yhfinance").Str("search_type", searchType).Str("search_string", searchString).Msg("Search performed")

			if searchType == "jump" {
				searchResultTicker, err := jumpSearch(ctx, searchString)
				if err != nil {
					*messages = append(*messages, Message{"sorry, error returned for that search", "danger"})
					break
				}
				if searchResultTicker.TickerSymbol == "" {
					*messages = append(*messages, Message{fmt.Sprintf("sorry, nothing found for '%s'", searchString), "warning"})
					break
				}
				zerolog.Ctx(ctx).Info().
					Str("search_provider", "yhfinance").
					Str("search_type", searchType).
					Str("search_string", searchString).
					Str("symbol", searchResultTicker.TickerSymbol).
					Msg("Search results")
				http.Redirect(w, r, fmt.Sprintf("/view/%s", searchResultTicker.TickerSymbol), http.StatusFound)
				return
			} else if searchType == "search" {
				searchResults, err := listSearch(ctx, searchString, "both")
				if err != nil {
					*messages = append(*messages, Message{"sorry, error returned for that search", "danger"})
					break
				}
				if len(searchResults) == 0 {
					*messages = append(*messages, Message{fmt.Sprintf("sorry, nothing found for '%s'", searchString), "warning"})
					break
				}
				zerolog.Ctx(ctx).Info().
					Str("search_provider", "yhfinance").
					Str("search_type", searchType).
					Str("search_string", searchString).
					Int("results_count", len(searchResults)).
					Msg("Search results")
				webdata["results"] = searchResults
				break
			} else {
				*messages = append(*messages, Message{"sorry, search type is unknown", "warning"})
				break
			}

		default:
			log.Warn().Str("search_type", searchType).Msg("Unknown search_type")
			*messages = append(*messages, Message{"sorry, invalid search request", "danger"})
		}

		renderTemplateDefault(w, r, "searchresults")
	})
}
