package main

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
)

func renderTemplateDefault(w http.ResponseWriter, r *http.Request, tmplname string, Data map[string]interface{}) {
	logger := log.Ctx(r.Context())
	tmpl, err := template.ParseGlob("templates/includes/_*.gohtml")
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to parse_glob template fragments")
		http.NotFound(w, r)
	}

	tmpl.ParseFiles("templates/" + tmplname + ".gohtml")
	if err != nil {
		logger.Warn().Err(err).Str("template", tmplname).Msg("Failed to parse template")
		http.NotFound(w, r)
	}

	config := Data["config"].(ConfigData)
	config.TmplName = tmplname
	Data["config"] = config

	err = tmpl.ExecuteTemplate(w, tmplname, Data)
	if err != nil {
		logger.Error().Err(err).
			Str("template", tmplname).
			Msg("Failed to execute template")
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
