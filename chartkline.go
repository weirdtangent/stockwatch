package main

import (
	"fmt"
	"html/template"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

type klineData struct {
	date string
	data [4]float32
}

func chartHandlerDailyKLine(ticker *Ticker, exchange *Exchange, dailies []Daily, webwatches []WebWatch) template.HTML {
	// build data needed
	days := len(dailies)
	x_axis := make([]string, 0, days)
	hidden_axis := make([]string, 0, days)
	candleData := make([]opts.KlineData, 0, days)
	volumeData := make([]opts.BarData, 0, days)
	for x := 0; x < days; x++ {
		displayDate := dailies[x].Price_date[5:10]
		x_axis = append(x_axis, displayDate)
		hidden_axis = append(hidden_axis, "")

		candleData = append(candleData, opts.KlineData{Value: [4]float32{dailies[x].Open_price, dailies[x].Close_price, dailies[x].Low_price, dailies[x].High_price}})
		volumeData = append(volumeData, opts.BarData{Value: dailies[x].Volume / 1000000})
	}

	// build charts
	prices := charts.NewKLine()
	prices.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:      "850px",
			Height:     "450px",
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s (%s) %s", ticker.Ticker_symbol, exchange.Exchange_acronym, ticker.Ticker_name),
			Subtitle: "Share Price",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Show: false,
			AxisLabel: &opts.AxisLabel{
				Show: false,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			AxisLabel: &opts.AxisLabel{
				Show: true,
			},
		}),
	)

	volume := charts.NewBar()
	volume.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:      "850px",
			Height:     "225px",
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTitleOpts(opts.Title{Subtitle: "Volume in mil"}),
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
		AddSeries("price", candleData, charts.WithLabelOpts(opts.Label{Show: false}))
	volume.SetXAxis(x_axis).
		AddSeries("volume", volumeData, charts.WithLabelOpts(opts.Label{Show: false}))

	prices.Renderer = newSnippetRenderer(prices, prices.Validate)
	volume.Renderer = newSnippetRenderer(volume, volume.Validate)

	return renderToHtml(prices) + renderToHtml(volume)
}
