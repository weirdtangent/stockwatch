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
	lineData := make([]opts.LineData, 0, 100)
	volumeData := make([]opts.BarData, 0)
	for x := 0; x < 100; x++ {
		priceDate = priceDate.AddDate(0, 0, 1)
		displayDate := priceDate.Format("2006-01-02")
		closePrice := dailies[x].Close_price
		x_axis = append(x_axis, displayDate)
		lineData = append(lineData, opts.LineData{Value: closePrice})
		volumeData = append(volumeData, opts.BarData{Value: dailies[x].Volume / 1000000})
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
		charts.WithLegendOpts(opts.Legend{
			Show:     true,
			Data:     []string{"Closeing Price", "5-Day MA", "10-Day MA", "20-Day MA"},
			Orient:   "horizontal",
			Selected: map[string]bool{ticker.Ticker_symbol: true, "MA5": false, "MA10": false, "MA20": false},
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Rotate: 45,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{}),
	)

	volume := charts.NewBar()
	volume.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "800px",
			Height: "225px",
			Theme:  types.ThemeVintage,
		}),
		charts.WithTitleOpts(opts.Title{
			Subtitle: "Volume in mil"}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Show:   false,
				Rotate: 45,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			AxisLabel: &opts.AxisLabel{
				Show: false,
			},
		}),
	)

	// Put data into instance
	prices.SetXAxis(x_axis).
		AddSeries(ticker.Ticker_symbol, lineData,
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA5", calcMovingAvg(5, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA10", calcMovingAvg(10, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA20", calcMovingAvg(20, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	volume.SetXAxis(x_axis).
		AddSeries("volume", volumeData, charts.WithLabelOpts(opts.Label{Show: false}))

	prices.Renderer = newSnippetRenderer(prices, prices.Validate)
	volume.Renderer = newSnippetRenderer(volume, volume.Validate)

	return renderToHtml(prices) + renderToHtml(volume)
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
