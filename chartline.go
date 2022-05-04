package main

import (
	"fmt"
	"html/template"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

func chartHandlerTickerDailyLine(deps *Dependencies, ticker Ticker, exchange *Exchange, dailies []TickerDaily, webwatches []WebWatch) template.HTML {
	nonce := deps.nonce
	sublog := deps.logger

	mainX := "700px"
	mainY := "280px"
	smallX := "700px"
	smallY := "200px"
	legendStrs := []string{"Closing Price", "20-Day MA", "50-Day MA", "200-Day MA"}

	// build data needed
	days := len(dailies)
	if days == 0 {
		html, _ := renderTemplateToString("_emptychart", nil)
		return html
	}

	x_axis := make([]string, 0, days)
	lineData := make([]opts.LineData, 0, days)
	volumeData := make([]opts.BarData, 0, days)
	for x := range dailies {
		// go or parseTime=true or something mysteriously turns the "string" PriceDate
		// which is yyyy-mm-dd into a full RFC3339 date, so we only want to parse the
		// first 10 characters
		tickerDate, err := time.Parse(sqlDateParseType, dailies[x].PriceDate[:10])
		if err != nil {
			sublog.Fatal().Err(err).Str("symbol", ticker.TickerSymbol).Str("bad_data", dailies[x].PriceDate).Msg("failed to parse price_date for {symbol}")
		}

		x_axis = append(x_axis, tickerDate.Format("Jan 02"))
		lineData = append(lineData, opts.LineData{Value: dailies[x].ClosePrice})
		volumeData = append(volumeData, opts.BarData{Value: dailies[x].Volume / volumeUnits})
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
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s/%s - %s", ticker.TickerSymbol, strings.ToLower(exchange.ExchangeAcronym), ticker.TickerName),
			Subtitle: "Share Price",
			Target:   nonce, // crazy hack to get nonce into scripts
		}),
		charts.WithLegendOpts(opts.Legend{
			Show:     true,
			Data:     legendStrs,
			Orient:   "horizontal",
			Selected: map[string]bool{ticker.TickerSymbol: true, "MA20": true, "MA50": true, "MA200": true},
			Left:     "center",
			Top:      "top",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Type: "category",
			Data: x_axis,
			AxisLabel: &opts.AxisLabel{
				Show:  false,
				Color: "white",
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Scale: true,
		}),
	)

	volume := charts.NewBar()
	volume.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:      smallX,
			Height:     smallY,
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
		}),
		charts.WithTitleOpts(opts.Title{
			Subtitle: "Volume in mil",
			Target:   nonce, // crazy hack to get nonce into scripts
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
	prices.SetXAxis(x_axis).
		AddSeries(ticker.TickerSymbol, lineData,
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	prices.
		AddSeries("MA20", calcMovingLineAvg(20, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}),
			charts.WithLineStyleOpts(opts.LineStyle{Width: 1}))
	prices.
		AddSeries("MA50", calcMovingLineAvg(50, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}),
			charts.WithLineStyleOpts(opts.LineStyle{Width: 1}))
	prices.
		AddSeries("MA200", calcMovingLineAvg(200, lineData),
			charts.WithLineChartOpts(opts.LineChart{Smooth: true}),
			charts.WithLineStyleOpts(opts.LineStyle{Width: 1}))

	volume.SetXAxis(x_axis).
		AddSeries("volume", volumeData, charts.WithLabelOpts(opts.Label{Show: false}))

	prices.Renderer = newSnippetRenderer(prices, prices.Validate)
	volume.Renderer = newSnippetRenderer(volume, volume.Validate)

	return renderToHtml(deps, prices) + renderToHtml(deps, volume)
}

// utils ----------------------------------------------------------------------

func calcMovingLineAvg(days float64, prices []opts.LineData) []opts.LineData {
	movingAvg := make([]opts.LineData, 0, 100)
	for i := range prices {
		if i < int(days) {
			movingAvg = append(movingAvg, opts.LineData{Value: "-"})
		} else {
			var sum float64 = 0
			for j := 0; j < int(days); j++ {
				val := reflect.ValueOf(prices[i-j])
				sum += val.FieldByName("Value").Interface().(float64)
			}
			movingAvg = append(movingAvg, opts.LineData{
				Value:  math.Round((sum/days)*100) / 100,
				Symbol: "none",
			})
		}
	}
	return movingAvg
}
