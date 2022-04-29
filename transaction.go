package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/weirdtangent/mymath"
)

type Transaction struct {
	TransactionId       uint64       `db:"transaction_id"`
	HoldingId           uint64       `db:"holding_id"`
	WatcherId           uint64       `db:"watcher_id"`
	TransactionType     string       `db:"transaction_type"`
	TransactionDateTime string       `db:"transaction_datetime"`
	Shares              uint64       `db:"shares"`
	SharePrice          float64      `db:"share_price"`
	CreateDatetime      sql.NullTime `db:"create_datetime"`
	UpdateDatetime      sql.NullTime `db:"update_datetime"`
}

func transactionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		webdata := ctx.Value(ContextKey("webdata")).(map[string]interface{})
		messages := ctx.Value(ContextKey("messages")).(*[]Message)

		watcher := webdata["watcher"].(Watcher)

		params := mux.Vars(r)
		action := params["action"]
		symbol := params["symbol"]
		acronym := params["acronym"]

		Shares, _ := strconv.ParseFloat(r.FormValue("Shares"), 64)
		SharePrice, _ := strconv.ParseFloat(r.FormValue("SharePrice"), 64)
		PurchaseDate := r.FormValue("PurchaseDate")

		shares, _ := mymath.FloatPrec(Shares, 2, 6)
		sharePrice, _ := mymath.FloatPrec(SharePrice, 2, 6)

		*messages = append(*messages, Message{fmt.Sprintf("Got it! Recorded that you %s %s shares of %s (%s) at $%s/share on %s",
			action, shares, symbol, acronym, sharePrice, PurchaseDate), "success"})
		zerolog.Ctx(ctx).Info().Uint64("watcher_id", watcher.WatcherId).Str("action", action).Float64("shares", Shares).Float64("share_price", SharePrice).Str("purchase_date", PurchaseDate).Str("symbol", symbol).Str("acronym", acronym).Msg("transaction recorded")

		renderTemplateDefault(w, r, "update")
	})
}
