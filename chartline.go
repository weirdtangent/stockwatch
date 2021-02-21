package main

import (
	"fmt"
	"html/template"
	"reflect"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	//"github.com/rs/zerolog/log"
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
	prices := charts.NewLine()
	prices.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "800px",
			Height: "350px",
			Theme:  types.ThemeVintage,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s (%s) %s", ticker.Ticker_symbol, exchange.Exchange_acronym, ticker.Ticker_name),
			Subtitle: "Share Price",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Rotate: 45,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{}),
	)

	// Put data into instance
	prices.SetXAxis(x_axis).
		AddSeries(ticker.Ticker_symbol, y_axis,
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA5", calcMovingAvg(5, y_axis),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA10", calcMovingAvg(10, y_axis),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA20", calcMovingAvg(20, y_axis),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	prices.Renderer = newSnippetRenderer(prices, prices.Validate)

	return renderToHtml(prices)
}

func calcMovingAvg(days float32, prices []opts.LineData) []opts.LineData {
	movingAvg := make([]opts.LineData, 0, 100)
	for i, _ := range prices {
		if i < int(days) {
			movingAvg = append(movingAvg, opts.LineData{Value: "-"})
		} else {
			var sum float32 = 0
			for j := 0; j < int(days); j++ {
				val := reflect.ValueOf(prices[i-j])
				sum += val.FieldByName("Value").Interface().(float32)
			}
			movingAvg = append(movingAvg, opts.LineData{Value: sum / days})
		}
	}
	return movingAvg
}
