package main

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
)

type DefaultView struct {
	Config   ConfigData
	Recents  []ViewPair
	Messages []Message
}

type TickerDailyView struct {
	Config         ConfigData
	Ticker         Ticker
	Exchange       Exchange
	Daily          Daily
	LastDailyMove  string
	Dailies        Dailies
	Watches        []WebWatch
	Recents        []ViewPair
	LineChartHTML  template.HTML
	KLineChartHTML template.HTML
	Messages       []Message
}

type TickerIntradayView struct {
	Config            ConfigData
	Ticker            Ticker
	Exchange          Exchange
	Daily             Daily
	LastDailyMove     string
	Intradate         string
	PriorBusinessDate string
	NextBusinessDate  string
	Intradays         Intradays
	Watches           []WebWatch
	Recents           []ViewPair
	LineChartHTML     template.HTML
	Messages          []Message
}

func renderTemplateDefault(w http.ResponseWriter, r *http.Request, tmplname string, Data map[string]interface{}) {
	tmpl, err := template.ParseFiles("templates/"+tmplname+".html", "templates/_wrapper.html")
	if err != nil {
		log.Warn().Err(err).Str("template", tmplname).Msg("Failed to parse template")
		http.NotFound(w, r)
	}

	config := Data["config"].(ConfigData)
	config.TmplName = tmplname
	Data["config"] = config

	err = tmpl.ExecuteTemplate(w, tmplname, Data)
	if err != nil {
		log.Error().Err(err).
			Str("template", tmplname).
			Msg("Failed to execute template")
	}
}

func renderTemplateToString(tmplname string, data interface{}) (template.HTML, error) {
	tmpl, err := template.ParseFiles("templates/"+tmplname+".html", "templates/_wrapper.html")
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

	//tpl := template.
	//	Must(template.New(tmplname).
	//		ParseFiles("templates/" + tmplname + ".html"),
	//	)
	//err := tpl.ExecuteTemplate(&html, tplName, r.c)

	return template.HTML(html.String()), nil
}
