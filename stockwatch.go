package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs"
	"github.com/alexedwards/scs/mysqlstore"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"

	"graystorm.com/myaws"
	"graystorm.com/mylog"
)

var sessionManager *scs.SessionManager
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

	// Initialize a new session manager and configure the session lifetime.
	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Store = mysqlstore.New(db_session.DB)
	sessionManager.Cookie.Domain = "stockwatch.graystorm.com"
}

func main() {
	mylog.Info.Print("Starting server on :3001")
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	mux.HandleFunc("/view/", viewHandler)
	mux.HandleFunc("/search/", searchHandler)
	mux.HandleFunc("/update/", updateHandler)
	mux.HandleFunc("/", homeHandler)

	// starup or die
	mylog.Error.Fatal(http.ListenAndServe(":3001", sessionManager.LoadAndSave(mux)))
}
