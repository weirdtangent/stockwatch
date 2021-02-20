package main

import (
	"html/template"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

func chartHandlerLine(ticker *Ticker, exchange *Exchange, dailies []Daily, webwatches []WebWatch) template.HTML {
	// build data needed
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
			Width: "1200px",
			Theme: types.ThemeVintage,
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      33,
			End:        100,
			XAxisIndex: []int{0},
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    ticker.Ticker_symbol,
			Subtitle: "Up to last 100 End of Day prices",
		}))

	// Put data into instance
	line.SetXAxis(x_axis).
		AddSeries(
			ticker.Ticker_symbol, y_axis).
		SetSeriesOptions(
			charts.WithLineChartOpts(
				opts.LineChart{Smooth: true}))

	line.Renderer = newSnippetRenderer(line, line.Validate)

	return renderToHtml(line)
}
