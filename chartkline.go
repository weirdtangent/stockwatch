package main

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	"github.com/rs/zerolog"
)

func chartHandlerTickerDailyKLine(ctx context.Context, ticker Ticker, exchange *Exchange, dailies []TickerDaily, webwatches []WebWatch) template.HTML {
	mainX := "880px"
	mainY := "280px"
	smallX := "880px"
	smallY := "200px"
	nonce := ctx.Value(ContextKey("nonce")).(string)

	// build data needed
	days := len(dailies)
	if days == 0 {
		html, _ := renderTemplateToString("_emptychart", nil)
		return html
	}

	x_axis := make([]string, 0, days)
	candleData := make([]opts.KlineData, 0, days)
	volumeData := make([]opts.BarData, 0, days)
	for x := range dailies {
		// go or parseTime=true or something mysteriously turns the "string" PriceDate
		// which is yyyy-mm-dd into a full RFC3339 date, so we only want to parse the
		// first 10 characters
		tickerDate, err := time.Parse(sqlDateType, dailies[x].PriceDate[:10])
		if err != nil {
			zerolog.Ctx(ctx).Fatal().Err(err).Str("symbol", ticker.TickerSymbol).Str("bad_data", dailies[x].PriceDate).Msg("failed to parse price_date for {symbol}")
		}

		x_axis = append(x_axis, tickerDate.Format("Jan 02"))
		candleData = append(candleData, opts.KlineData{Value: [4]float64{dailies[x].OpenPrice, dailies[x].ClosePrice, dailies[x].LowPrice, dailies[x].HighPrice}})
		volumeData = append(volumeData, opts.BarData{Value: dailies[x].Volume / volumeUnits})
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
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s/%s - %s", ticker.TickerSymbol, strings.ToLower(exchange.ExchangeAcronym), ticker.TickerName),
			Subtitle: "Share Price",
			Target:   nonce, // crazy hack to get nonce into scripts
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Show: false,
			AxisLabel: &opts.AxisLabel{
				Show:  false,
				Color: "white",
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			AxisLabel: &opts.AxisLabel{
				Show: true,
			},
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
		AddSeries(ticker.TickerSymbol, candleData,
			charts.WithLabelOpts(opts.Label{Show: false}),
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color:        "green",
				Color0:       "red",
				BorderColor:  "green",
				BorderColor0: "red",
			}),
		)
	volume.SetXAxis(x_axis).
		AddSeries("volume", volumeData,
			charts.WithLabelOpts(opts.Label{
				Show: false}),
		)

	prices.Renderer = newSnippetRenderer(prices, prices.Validate)
	volume.Renderer = newSnippetRenderer(volume, volume.Validate)

	return renderToHtml(ctx, prices) + renderToHtml(ctx, volume)
}
