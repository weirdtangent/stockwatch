package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Transaction struct {
	TransactionId       int64   `db:"transaction_id"`
	HoldingId           int64   `db:"holding_id"`
	WatcherId           int64   `db:"watcher_id"`
	TransactionType     string  `db:"transaction_type"`
	TransactionDateTime string  `db:"transaction_datetime"`
	Shares              int64   `db:"shares"`
	SharePrice          float64 `db:"share_price"`
	CreateDatetime      string  `db:"create_datetime"`
	UpdateDatetime      string  `db:"update_datetime"`
}

func transactionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		webdata := ctx.Value("webdata").(map[string]interface{})

		messages := make([]Message, 0)

		// only authenticate can record bought or sold
		if ok := checkAuthState(w, r); ok == false {
			http.NotFound(w, r)
		} else {
			params := mux.Vars(r)
			action := params["action"]
			submit := params["submit"]

			if submit == "" {
				webdata["messages"] = Messages{messages}
				renderTemplateDefault(w, r, action, webdata)
			} else {

			}

		}
		return
	})
}
