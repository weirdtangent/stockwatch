package main

import (
	"fmt"
	"net/http"

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

func searchHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata
		sublog := deps.logger

		checkAuthState(w, r, deps)
		// if ctx, ok := checkAuthState(w, r, deps); !ok {
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
				break
			}
			webdata["searchString"] = searchString

			sublog.Info().Str("search_provider", "yhfinance").Str("search_type", searchType).Str("search_string", searchString).Msg("Search performed")

			if searchType == "jump" {
				searchResultTicker, err := jumpSearch(deps, searchString)
				if err != nil || searchResultTicker.TickerSymbol == "" {
					break
				}
				http.Redirect(w, r, fmt.Sprintf("/view/%s", searchResultTicker.TickerSymbol), http.StatusFound)
				return
			} else if searchType == "search" {
				searchResults, err := listSearch(deps, searchString, "both")
				if err != nil || len(searchResults) == 0 {
					break
				}
				webdata["results"] = searchResults
			}

		default:
			log.Warn().Str("search_type", searchType).Msg("Unknown search_type")
		}

		renderTemplate(w, r, deps, "searchresults")
	})
}
