package stockwatch

import (
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
			mylog.Error.Fatal(err)
		}
		if success != true {
			mylog.Info.Print("exchanges update failed")
		}
	case "ticker":
		symbol := params[1]
		mylog.Info.Printf("ok doing update of ticker symbol %s", symbol)
		_, err := updateMarketstackTicker(symbol)
		if err != nil {
			mylog.Error.Fatal(err)
		}
	case "dummy":
		mylog.Info.Print("just show the template")
	default:
		mylog.Error.Fatal("unknown update action: " + action)
	}

	var data = NilView{}
	renderTemplateMessages(w, r, "update", &data)
}
