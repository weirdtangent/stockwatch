package main

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
)

func renderTemplateDefault(w http.ResponseWriter, r *http.Request, deps *Dependencies, tmplname string) {
	webdata := deps.webdata
	config := deps.config
	sublog := deps.logger

	config["template_name"] = tmplname
	webdata["config"] = config

	tmpl := deps.templates

	err := tmpl.ExecuteTemplate(w, tmplname, webdata)
	if err != nil {
		sublog.Error().Err(err).Str("template", tmplname).Msg("Failed to execute template")
	}
}

func renderTemplateToString(tmplname string, data interface{}) (template.HTML, error) {
	tmpl, err := template.ParseFiles("templates/" + tmplname + ".gohtml")
	if err != nil {
		log.Warn().Err(err).
			Str("template", tmplname).
			Msg("Failed to parse template")
		return "", err
	}

	var html bytes.Buffer
	err = tmpl.ExecuteTemplate(&html, tmplname, nil)
	if err != nil {
		log.Warn().Err(err).
			Str("template", tmplname).
			Msg("Failed to execute template")
		return "", err
	}

	return template.HTML(html.String()), nil
}
