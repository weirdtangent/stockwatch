package main

import (
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
)

func renderTemplate(w http.ResponseWriter, r *http.Request, tmplname string) {
	tmpl, err := template.ParseFiles("templates/"+tmplname+".html", "templates/wrapper.html")
	if err != nil {
		log.Warn().Err(err).Str("template", tmplname).Msg("Failed to parse template")
		http.NotFound(w, r)
	}

	var Empty interface{}
	err = tmpl.ExecuteTemplate(w, tmplname, &Empty)
	if err != nil {
		log.Error().Err(err).Str("template", tmplname).Msg("Failed to execute template")
	}
}

func renderTemplateMessages(w http.ResponseWriter, r *http.Request, tmplname string, data *Message) {
	tmpl, err := template.ParseFiles("templates/"+tmplname+".html", "templates/wrapper.html")
	if err != nil {
		log.Warn().Err(err).Str("template", tmplname).Msg("Failed to parse template")
		http.NotFound(w, r)
	}

	err = tmpl.ExecuteTemplate(w, tmplname, data)
	if err != nil {
		log.Error().Err(err).Str("template", tmplname).Msg("Failed to execute template")
	}
}

func renderTemplateView(w http.ResponseWriter, r *http.Request, tmplname string, data *TickerView) {
	tmpl, err := template.ParseFiles("templates/"+tmplname+".html", "templates/wrapper.html")
	if err != nil {
		log.Warn().Err(err).Str("template", tmplname).Msg("Failed to parse template")
		http.NotFound(w, r)
	}

	err = tmpl.ExecuteTemplate(w, tmplname, data)
	if err != nil {
		log.Error().Err(err).Str("template", tmplname).Msg("Failed to execute template")
	}
}
