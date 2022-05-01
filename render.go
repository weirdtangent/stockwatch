package main

import (
	"bytes"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

func renderTemplateDefault(w http.ResponseWriter, r *http.Request, deps *Dependencies, tmplname string) {
	config := deps.config
	webdata := deps.webdata
	// messages := deps.messages
	sublog := deps.logger

	config["template_name"] = tmplname

	// webdata["messages"] = messages
	webdata["config"] = config

	funcMap := template.FuncMap{
		"FormatUnixTime":        FormatUnixTime,
		"GradeColor":            GradeColor,
		"SinceColor":            SinceColor,
		"PriceDiffAmt":          PriceDiffAmt,
		"PriceDiffPercAmt":      PriceDiffPercAmt,
		"PriceMoveColor":        PriceMoveColorCSS,
		"PriceBigMoveColor":     PriceBigMoveColorCSS,
		"PriceMoveIndicator":    PriceMoveIndicatorCSS,
		"PriceBigMoveIndicator": PriceBigMoveIndicatorCSS,
		"TimeNow":               TimeNow,
		"ToUpper":               strings.ToUpper,
		"ToLower":               strings.ToLower,
	}

	tmpl := template.New("blank").Funcs(funcMap)
	tmpl, err := tmpl.ParseGlob("templates/includes/*.gohtml")
	if err != nil {
		sublog.Fatal().Err(err).Str("template_dir", "includes").Msg("Failed to parse template(s)")
	}
	tmpl, err = tmpl.ParseGlob("templates/modals/*.gohtml")
	if err != nil {
		sublog.Fatal().Err(err).Str("template_dir", "modals").Msg("Failed to parse template(s)")
	}
	// Parse variable "about" page into template
	if val, ok := webdata["about-contents_template"]; ok {
		tmpl, err = tmpl.Parse("{{ define \"about-contents\" }}" + *val.(*string) + "{{end}}")
		if err != nil {
			sublog.Fatal().Err(err).Msg("Failed to parse 'about' page into template")
		}
	}
	// Parse all internal articles as templates
	article_rx := regexp.MustCompile(`_source.*body_template`)
	for webTmpl, val := range webdata {
		if article_rx.Match([]byte(webTmpl)) {
			template.Must(tmpl.New(webTmpl).Parse(val.(WebArticle).Body))
		}
	}
	tmpl, err = tmpl.ParseFiles("templates/" + tmplname + ".gohtml")
	if err != nil {
		sublog.Fatal().Err(err).Str("template", tmplname).Msg("Failed to parse template")
	}
	err = tmpl.ExecuteTemplate(w, tmplname, webdata)
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
