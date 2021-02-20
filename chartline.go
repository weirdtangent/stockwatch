package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"time"

	"graystorm.com/mylog"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	chartrender "github.com/go-echarts/go-echarts/v2/render"
	"github.com/go-echarts/go-echarts/v2/types"
)

func chartHandlerLine(ticker *Ticker, exchange *Exchange, dailies []Daily, webwatches []WebWatch) template.HTML {
	EasternTZ, _ := time.LoadLocation("America/New_York")
	endDate := time.Now().In(EasternTZ)
	priceDate := endDate.AddDate(0, 0, -100)

	x_axis := make([]string, 0, 100)
	y_axis := make([]opts.LineData, 0, 100)
	for x := 0; x < 100; x++ {
		priceDate = priceDate.AddDate(0, 0, 1)
		displayDate := priceDate.Format("2006-01-02")
		closePrice := dailies[x].Close_price
		x_axis = append(x_axis, displayDate)
		y_axis = append(y_axis, opts.LineData{Value: closePrice})
	}

	// construct line chart
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width: "600px",
			Theme: types.ThemeVintage,
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      50,
			End:        100,
			XAxisIndex: []int{0},
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    ticker.Ticker_symbol,
			Subtitle: "Up to last 100 End of Day prices",
		}))

	// Put data into instance
	line.SetXAxis(x_axis).
		AddSeries(ticker.Ticker_symbol, y_axis).
		SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	line.Renderer = newSnippetRenderer(line, line.Validate)

	return renderToHtml(line)
}

func renderToHtml(c interface{}) template.HTML {
	var buf bytes.Buffer
	r := c.(chartrender.Renderer)
	err := r.Render(&buf)
	if err != nil {
		mylog.Error.Printf("Failed to render chart: %s", err)
		return ""
	}

	return template.HTML(buf.String())
}

// adapted from
// https://github.com/go-echarts/go-echarts/blob/master/templates/base.go
// https://github.com/go-echarts/go-echarts/blob/master/templates/header.go
var baseTpl = `
<div class="container">
    <div class="item" id="{{ .ChartID }}" style="width:{{ .Initialization.Width }};height:{{ .Initialization.Height }};"></div>
</div>
<script type="text/javascript">
    "use strict";
    let goecharts_{{ .ChartID | safeJS }} = echarts.init(document.getElementById('{{ .ChartID | safeJS }}'), "{{ .Theme }}");
    let option_{{ .ChartID | safeJS }} = {{ .JSON }};
    goecharts_{{ .ChartID | safeJS }}.setOption(option_{{ .ChartID | safeJS }});
    {{- range .JSFunctions.Fns }}
    {{ . | safeJS }}
    {{- end }}
</script>
`

type snippetRenderer struct {
	c      interface{}
	before []func()
}

func newSnippetRenderer(c interface{}, before ...func()) chartrender.Renderer {
	return &snippetRenderer{c: c, before: before}
}

func (r *snippetRenderer) Render(w io.Writer) error {
	const tplName = "chart"
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
			Parse(baseTpl),
		)

	err := tpl.ExecuteTemplate(w, tplName, r.c)
	return err
}
