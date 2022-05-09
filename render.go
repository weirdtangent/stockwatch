package main

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/rs/zerolog"
)

func staticPageHandler(deps *Dependencies, tmplname string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sublog := deps.logger

		checkAuthState(w, r, deps)
		webdata := deps.webdata

		if tmplname == "home" || tmplname == "terms" || tmplname == "privacy" {
			webdata["hideRecents"] = true
		}
		if tmplname == "about" {
			webdata["about"], webdata["commits"], _ = getGithubCommits(deps)
		}

		renderTemplate(w, r, deps, *sublog, tmplname)
	})
}

// renderTemplate is a wrapper around template.ExecuteTemplate.
// It writes into a bytes.Buffer before writing to the http.ResponseWriter to catch
// any errors resulting from populating the template.
func renderTemplate(w http.ResponseWriter, r *http.Request, deps *Dependencies, sublog zerolog.Logger, tmplname string) error {
	tmpl := deps.templates
	config := deps.config
	webdata := deps.webdata

	config["template_name"] = tmplname
	webdata["config"] = config
	webdata["messages"] = deps.messages
	webdata["nonce"] = deps.nonce

	// Create a buffer to temporarily write to and check if any errors were encountered.
	buf := deps.bufpool.Get()
	defer deps.bufpool.Put(buf)

	err := tmpl.ExecuteTemplate(buf, tmplname, webdata)
	if err != nil {
		sublog.Error().Err(err).Str("template", tmplname).Msg("failed to execute template")
		return err
	}

	// Set the header and write the buffer to the http.ResponseWriter
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
	return nil
}

func renderTemplateToString(deps *Dependencies, tmplname string, data interface{}) (template.HTML, error) {
	tmpl := deps.templates
	sublog := deps.logger

	// Create a buffer to temporarily write to and check if any errors were encountered.
	buf := deps.bufpool.Get()
	defer deps.bufpool.Put(buf)

	err := tmpl.ExecuteTemplate(buf, tmplname, nil)
	if err != nil {
		sublog.Error().Err(err).Str("template", tmplname).Msg("failed to execute template")
		return "", err
	}

	var html bytes.Buffer
	html.Write(buf.Bytes())

	return template.HTML(html.String()), nil
}
