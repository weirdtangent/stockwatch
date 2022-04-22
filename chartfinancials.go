package main

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

func chartHandlerFinancialsBar(ctx context.Context, ticker *Ticker, exchange *Exchange, periodStrs []string, barValues []map[string]float64) template.HTML {
	mainX := "880px"
	mainY := "400px"
	nonce := ctx.Value(ContextKey("nonce")).(string)

	// acctg := accounting.Accounting{Symbol: "$", Precision: 0}

	var barData = map[string][]opts.BarData{}
	var legendStrs = []string{}
	for x := range periodStrs {
		for category, value := range barValues[x] {
			if x == 0 {
				legendStrs = append(legendStrs, category)
			}
			barData[category] = append(barData[category], opts.BarData{Name: category, Value: value / 1000000})
		}
	}

	// construct bar chart
	barChart := charts.NewBar()
	barChart.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:      mainX,
			Height:     mainY,
			Theme:      types.ThemeVintage,
			AssetsHost: "https://stockwatch.graystorm.com/static/vendor/echarts/dist/",
		}),
		charts.WithTitleOpts(opts.Title{
			Title: fmt.Sprintf("%s/%s - %s", ticker.TickerSymbol, strings.ToLower(exchange.ExchangeAcronym), ticker.TickerName),
			// Subtitle: "Quarterly Financials",
			Target: nonce, // crazy hack to get nonce into scripts
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show:   true,
			Data:   legendStrs,
			Orient: "horizontal",
			Left:   "center",
			Top:    "bottom",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name:      "Period",
			Type:      "category",
			Show:      true,
			Data:      periodStrs,
			AxisLabel: &opts.AxisLabel{Show: true},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Type:      "value",
			Name:      "in Millions of $",
			Scale:     false,
			AxisLabel: &opts.AxisLabel{Show: true},
		}),
	)

	// Put data into instance
	// barChart.SetXAxis(quarterStrs)
	for category, data := range barData {
		barChart.
			SetXAxis(periodStrs).
			AddSeries(
				category,
				data,
				charts.WithBarChartOpts(opts.BarChart{Type: "bar", BarGap: "5%", BarCategoryGap: "25%"}),
			)
	}

	barChart.Renderer = newSnippetRenderer(barChart, barChart.Validate)

	return renderToHtml(barChart)
}
