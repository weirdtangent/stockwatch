package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
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

// object methods -------------------------------------------------------------

// misc -----------------------------------------------------------------------

func searchHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webdata := deps.webdata

		checkAuthState(w, r, deps, *deps.logger)

		params := mux.Vars(r)
		searchType := params["type"]

		sublog := deps.logger.With().Str("search_type", searchType).Logger()

		switch searchType {
		case "ticker":
			searchString := r.FormValue("searchString")
			searchType := r.FormValue("submit")
			if searchString == "" || searchType == "" {
				break
			}
			webdata["searchString"] = searchString

			sublog.Info().Str("search_provider", "yhfinance").Str("search_string", searchString).Msg("Search performed")

			if searchType == "jump" {
				searchResultTicker, err := jumpSearch(deps, sublog, searchString)
				if err != nil || searchResultTicker.TickerSymbol == "" {
					break
				}
				http.Redirect(w, r, fmt.Sprintf("/view/%s", searchResultTicker.TickerSymbol), http.StatusFound)
				return
			} else if searchType == "search" {
				searchResults, err := listSearch(deps, sublog, searchString, "both")
				if err != nil || len(searchResults) == 0 {
					break
				}
				webdata["results"] = searchResults
			}

		default:
			sublog.Warn().Msg("Unknown search_type")
		}

		renderTemplate(w, r, deps, sublog, "searchresults")
	})
}
