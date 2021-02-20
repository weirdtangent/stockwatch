package main

import (
	"fmt"
	"net/http"
	"strings"

	"graystorm.com/mylog"
)

func updateHandler(w http.ResponseWriter, r *http.Request) {
	path_paramlist := r.URL.Path[len("/update/"):]
	params := strings.Split(path_paramlist, "/")
	action := params[0]

	switch action {
	case "exchanges":
		mylog.Info.Print("ok doing update of exchanges")
		success, err := updateMarketstackExchanges()
		if err != nil {
			errorHandler(w, r, fmt.Sprintf("Bulk update of Exchanges failed: %s", err))
			return
		}
		if success != true {
			errorHandler(w, r, "Bulk update of Exchanges failed")
			return
		}
	case "ticker":
		symbol := params[1]
		_, err := updateMarketstackTicker(symbol)
		if err != nil {
			errorHandler(w, r, fmt.Sprintf("Update of ticket symbol %s failed: %s", symbol, err))
			return
		}
	case "dummy":
		mylog.Info.Print("just show the template")
	default:
		mylog.Error.Fatal("unknown update action: " + action)
	}

	errorHandler(w, r, "Operation completed normally")
}
