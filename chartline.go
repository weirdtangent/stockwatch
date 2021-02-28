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

const (
	mainX  = "750px"
	mainY  = "350px"
	smallX = "750px"
	smallY = "225px"
)

func chartHandlerDailyLine(ticker *Ticker, exchange *Exchange, dailies []Daily, webwatches []WebWatch) template.HTML {
	// build data needed
	days := len(dailies)
	if days == 0 {
		html, _ := renderTemplateToString("_emptychart", nil)
		return html
	}
	x_axis := make([]string, 0, days)
	hidden_axis := make([]string, 0, days)
	lineData := make([]opts.LineData, 0, days)
	volumeData := make([]opts.BarData, 0, days)
	for x := range dailies {
		displayDate := dailies[x].PriceDate[5:10]
		closePrice := dailies[x].ClosePrice

		x_axis = append(x_axis, displayDate)
		hidden_axis = append(hidden_axis, "")
		lineData = append(lineData, opts.LineData{Value: closePrice})
		volumeData = append(volumeData, opts.BarData{Value: dailies[x].Volume / 1000000})
	}

	// construct line chart
	prices := charts.NewLine()
	prices.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:      mainX,
			Height:     mainY,
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s (%s) %s", ticker.TickerSymbol, exchange.ExchangeAcronym, ticker.TickerName),
			Subtitle: "Share Price",
			Target:   global_nonce, // crazy hack to get nonce into scripts
		}),
		charts.WithLegendOpts(opts.Legend{
			Show:     true,
			Data:     []string{"Closeing Price", "5-Day MA", "10-Day MA", "20-Day MA"},
			Orient:   "horizontal",
			Selected: map[string]bool{ticker.TickerSymbol: true, "MA5": false, "MA10": false, "MA20": false},
			Left:     "right",
			Top:      "top",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Show: false,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{}),
	)

	volume := charts.NewBar()
	volume.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:      smallX,
			Height:     smallY,
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTitleOpts(opts.Title{
			Subtitle: "Volume in mil",
			Target:   global_nonce, // crazy hack to get nonce into scripts
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Rotate: 45,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Show: false,
			AxisLabel: &opts.AxisLabel{
				Show: false,
			},
		}),
	)

	// Put data into instance
	prices.SetXAxis(hidden_axis).
		AddSeries(ticker.TickerSymbol, lineData,
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA5", calcMovingLineAvg(5, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA10", calcMovingLineAvg(10, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA20", calcMovingLineAvg(20, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	volume.SetXAxis(x_axis).
		AddSeries("volume", volumeData, charts.WithLabelOpts(opts.Label{Show: false}))

	prices.Renderer = newSnippetRenderer(prices, prices.Validate)
	volume.Renderer = newSnippetRenderer(volume, volume.Validate)

	return renderToHtml(prices) + renderToHtml(volume)
}

func chartHandlerIntradayLine(ticker *Ticker, exchange *Exchange, intradays []Intraday, webwatches []WebWatch, intradate string) template.HTML {
	// build data needed
	steps := len(intradays)
	if steps == 0 {
		html, _ := renderTemplateToString("_emptychart", nil)
		return html
	}
	x_axis := make([]string, 0, steps)
	hidden_axis := make([]string, 0, steps)
	lineData := make([]opts.LineData, 0, steps)
	volumeData := make([]opts.BarData, 0, steps)
	EasternTZ, _ := time.LoadLocation("America/New_York")
	for x := range intradays {
		datePart := intradays[x].PriceDate[0:10]
		var displayDate string
		if datePart != intradate {
			displayDate = datePart
		} else {
			timeSlot, _ := time.Parse("15:04", intradays[x].PriceDate[11:16])
			displayDate = timeSlot.In(EasternTZ).Format("15:04")
		}
		closePrice := intradays[x].LastPrice

		x_axis = append(x_axis, displayDate)
		hidden_axis = append(hidden_axis, "")
		lineData = append(lineData, opts.LineData{Value: closePrice})
		volumeData = append(volumeData, opts.BarData{Value: intradays[x].Volume})
	}

	// construct line chart
	prices := charts.NewLine()
	prices.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:      mainX,
			Height:     mainY,
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s (%s) %s", ticker.TickerSymbol, exchange.ExchangeAcronym, ticker.TickerName),
			Subtitle: "Share Price for " + intradate,
			Target:   global_nonce, // crazy hack to get nonce into scripts
		}),
		charts.WithLegendOpts(opts.Legend{
			Show:     true,
			Data:     []string{"Closeing Price", "5-Day MA", "10-Day MA", "20-Day MA"},
			Orient:   "horizontal",
			Selected: map[string]bool{ticker.TickerSymbol: true, "MA5": false, "MA10": false, "MA20": false},
			Left:     "right",
			Top:      "top",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Show: false,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{}),
	)

	volume := charts.NewBar()
	volume.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:      smallX,
			Height:     smallY,
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTitleOpts(opts.Title{
			Subtitle: "Volume",
			Target:   global_nonce, // crazy hack to get nonce into scripts
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Rotate: 60,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Show: false,
			AxisLabel: &opts.AxisLabel{
				Show: false,
			},
		}),
	)

	// Put data into instance
	prices.SetXAxis(hidden_axis).
		AddSeries(ticker.TickerSymbol, lineData)
	prices.
		AddSeries("MA5", calcMovingLineAvg(5, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA10", calcMovingLineAvg(10, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA20", calcMovingLineAvg(20, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	volume.SetXAxis(x_axis).
		AddSeries("volume", volumeData, charts.WithLabelOpts(opts.Label{Show: false}))

	prices.Renderer = newSnippetRenderer(prices, prices.Validate)
	volume.Renderer = newSnippetRenderer(volume, volume.Validate)

	return renderToHtml(prices) + renderToHtml(volume)
}

// utils ----------------------------------------------------------------------

func calcMovingLineAvg(days float32, prices []opts.LineData) []opts.LineData {
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
