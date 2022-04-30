package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"

	chartrender "github.com/go-echarts/go-echarts/v2/render"
)

func renderToHtml(deps *Dependencies, c interface{}) template.HTML {
	sublog := deps.logger

	var buf bytes.Buffer
	r := c.(chartrender.Renderer)
	err := r.Render(&buf)
	if err != nil {
		sublog.Error().Err(err).Msg("Failed to render line chart")
		return ""
	}

	return template.HTML(buf.String())
}

type snippetRenderer struct {
	c interface{}
	// nonce  string
	before []func()
}

func newSnippetRenderer(c interface{}, before ...func()) chartrender.Renderer {
	return &snippetRenderer{c: c, before: before}
}

func (r *snippetRenderer) Render(w io.Writer) error {
	const tplName = "_chart"
	for _, fn := range r.before {
		fn()
	}

	tpl := template.
		Must(template.New(tplName).
			Funcs(template.FuncMap{
				"safeJS": func(s interface{}) template.JS {
					return template.JS(fmt.Sprint(s))
				},
			}).
			ParseFiles("templates/charts/_chart.gohtml"),
		)

	err := tpl.ExecuteTemplate(w, tplName, r.c)
	return err
}
