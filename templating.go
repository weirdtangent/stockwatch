package main

import (
	"html/template"
	"net/http"

	"graystorm.com/mylog"
)

func renderTemplate(w http.ResponseWriter, r *http.Request, tmplname string) {
	tmpl, err := template.ParseFiles("templates/header.html", "templates/footer.html", "templates/"+tmplname+".html")
	if err != nil {
		mylog.Warning.Print(err)
		http.NotFound(w, r)
	}

	err = tmpl.ExecuteTemplate(w, tmplname, NilView{})
	if err != nil {
		mylog.Error.Print(err)
	}
}

func renderTemplateView(w http.ResponseWriter, r *http.Request, tmplname string, data *TickerView) {
	tmpl, err := template.ParseFiles("templates/header.html", "templates/footer.html", "templates/"+tmplname+".html")
	if err != nil {
		mylog.Warning.Print(err)
		http.NotFound(w, r)
	}

	err = tmpl.ExecuteTemplate(w, tmplname, data)
	if err != nil {
		mylog.Error.Print(err)
	}
}

func renderTemplateMessages(w http.ResponseWriter, r *http.Request, tmplname string, data *Message) {
	tmpl, err := template.ParseFiles("templates/header.html", "templates/footer.html", "templates/"+tmplname+".html")
	if err != nil {
		mylog.Warning.Print(err)
		http.NotFound(w, r)
	}

	err = tmpl.ExecuteTemplate(w, tmplname, data)
	if err != nil {
		mylog.Error.Print(err)
	}
}
