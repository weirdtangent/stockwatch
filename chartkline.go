package main

import (
	"context"
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

func chartHandlerTickerDailyKLine(ctx context.Context, ticker *Ticker, exchange *Exchange, dailies []TickerDaily, webwatches []WebWatch) template.HTML {
	nonce := ctx.Value("nonce").(string)

	// build data needed
	days := len(dailies)
	if days == 0 {
		html, _ := renderTemplateToString("_emptychart", nil)
		return html
	}
	x_axis := make([]string, 0, days)
	hidden_axis := make([]string, 0, days)
	candleData := make([]opts.KlineData, 0, days)
	volumeData := make([]opts.BarData, 0, days)
	for x := range dailies {
		displayDate := dailies[x].PriceDate[5:10]
		x_axis = append(x_axis, displayDate)
		hidden_axis = append(hidden_axis, "")

		candleData = append(candleData, opts.KlineData{Value: [4]float32{dailies[x].OpenPrice, dailies[x].ClosePrice, dailies[x].LowPrice, dailies[x].HighPrice}})
		volumeData = append(volumeData, opts.BarData{Value: dailies[x].Volume / 1000000})
	}

	// build charts
	prices := charts.NewKLine()
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
			Target:   nonce, // crazy hack to get nonce into scripts
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
			Width:      smallX,
			Height:     smallY,
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTitleOpts(opts.Title{
			Subtitle: "Volume in mil",
			Target:   nonce, // crazy hack to get nonce into scripts
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
		AddSeries("price", candleData, charts.WithLabelOpts(opts.Label{Show: false}))
	volume.SetXAxis(x_axis).
		AddSeries("volume", volumeData, charts.WithLabelOpts(opts.Label{Show: false}))

	prices.Renderer = newSnippetRenderer(prices, prices.Validate)
	volume.Renderer = newSnippetRenderer(volume, volume.Validate)

	return renderToHtml(prices) + renderToHtml(volume)
}
