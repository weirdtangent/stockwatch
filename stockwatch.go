package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"

	"graystorm.com/myaws"
	"graystorm.com/mylog"
)

var aws_session *session.Session
var db_session *sqlx.DB

func init() {
	// initialize logging calls
	mylog.Init(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)

	// connect to AWS
	var err error
	aws_session, err = myaws.AWSConnect("us-east-1", "stockwatch")
	if err != nil {
		mylog.Error.Fatal(err)
	}

	// connect to Aurora
	db_session, err = myaws.DBConnect(aws_session, "stockwatch_rds", "stockwatch")
	if err != nil {
		fmt.Print(err.Error())
	}
}

func main() {
	mylog.Info.Print("Starting server on :3001")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	http.HandleFunc("/view/", viewHandler)
	http.HandleFunc("/search/", searchHandler)
	http.HandleFunc("/update/", updateHandler)
	http.HandleFunc("/", homeHandler)

	// starup or die
	mylog.Error.Fatal(http.ListenAndServe(":3001", nil))
}
