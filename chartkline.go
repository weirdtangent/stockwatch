package main

import (
	"fmt"
	"html/template"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

type klineData struct {
	date string
	data [4]float32
}

func chartHandlerKLine(ticker *Ticker, exchange *Exchange, dailies []Daily, webwatches []WebWatch) template.HTML {
	// build data needed
	EasternTZ, _ := time.LoadLocation("America/New_York")
	endDate := time.Now().In(EasternTZ)
	priceDate := endDate.AddDate(0, 0, -100)

	x_axis := make([]string, 0)
	candleData := make([]opts.KlineData, 0)
	volumeData := make([]opts.BarData, 0)
	for x := 0; x < len(dailies); x++ {
		priceDate = priceDate.AddDate(0, 0, 1)
		x_axis = append(x_axis, priceDate.Format("2006-01-02"))
		candleData = append(candleData, opts.KlineData{Value: [4]float32{dailies[x].Open_price, dailies[x].Close_price, dailies[x].Low_price, dailies[x].High_price}})
		volumeData = append(volumeData, opts.BarData{Value: dailies[x].Volume})
	}

	// build charts
	prices := charts.NewKLine()
	prices.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width: "1200px",
			Theme: types.ThemeVintage,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s (%s) %s", ticker.Ticker_symbol, exchange.Exchange_acronym, ticker.Ticker_name),
			Subtitle: "Share Price",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Show:   false,
				Rotate: 45,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{}),
	)

	volume := charts.NewBar()
	volume.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1200px",
			Height: "200px",
			Theme:  types.ThemeVintage,
		}),
		charts.WithTitleOpts(opts.Title{
			Subtitle: "Volume",
		}),
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
		AddSeries("price", candleData)
	prices.Renderer = newSnippetRenderer(prices, prices.Validate)

	volume.SetXAxis(x_axis).
		AddSeries("volume", volumeData)
	volume.Renderer = newSnippetRenderer(volume, volume.Validate)

	return renderToHtml(prices) + renderToHtml(volume)
}
