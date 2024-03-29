package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type Transaction struct {
	TransactionId       uint64 `db:"transaction_id"`
	EId                 string
	HoldingId           uint64    `db:"holding_id"`
	WatcherId           uint64    `db:"watcher_id"`
	TransactionType     string    `db:"transaction_type"`
	TransactionDateTime string    `db:"transaction_datetime"`
	Shares              uint64    `db:"shares"`
	SharePrice          float64   `db:"share_price"`
	CreateDatetime      time.Time `db:"create_datetime"`
	UpdateDatetime      time.Time `db:"update_datetime"`
}

// object methods -------------------------------------------------------------

// misc -----------------------------------------------------------------------

func transactionHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		watcher := checkAuthState(w, r, deps, *deps.logger)

		params := mux.Vars(r)
		action := params["action"]
		symbol := params["symbol"]
		acronym := params["acronym"]

		sublog := deps.logger.With().Str("watcher", watcher.EId).Str("transaction", action).Str("symbol", symbol).Logger()

		Shares, _ := strconv.ParseFloat(r.FormValue("Shares"), 64)
		SharePrice, _ := strconv.ParseFloat(r.FormValue("SharePrice"), 64)
		PurchaseDate := r.FormValue("PurchaseDate")

		sublog.Info().Float64("shares", Shares).Float64("share_price", SharePrice).Str("purchase_date", PurchaseDate).Str("acronym", acronym).Msg("transaction recorded")

		renderTemplate(w, r, deps, sublog, "update")
	})
}
