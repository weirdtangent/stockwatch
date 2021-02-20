package main

import (
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
	EasternTZ, _ := time.LoadLocation("America/New_York")
	endDate := time.Now().In(EasternTZ)
	priceDate := endDate.AddDate(0, 0, -100)

	// construct line chart
	kline := charts.NewKLine()
	x_axis := make([]string, 0)
	y_axis := make([]opts.KlineData, 0)
	for x := 0; x < len(dailies); x++ {
		priceDate = priceDate.AddDate(0, 0, 1)
		x_axis = append(x_axis, priceDate.Format("2006-01-02"))
		y_axis = append(y_axis, opts.KlineData{Value: [4]float32{dailies[x].Open_price, dailies[x].High_price, dailies[x].Low_price, dailies[x].Close_price}})
	}

	kline.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width: "600px",
			Theme: types.ThemeVintage,
		}),
		charts.WithTitleOpts(opts.Title{
			Title: ticker.Ticker_symbol,
		}),
		charts.WithXAxisOpts(opts.XAxis{
			SplitNumber: 20,
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Scale: true,
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      50,
			End:        100,
			XAxisIndex: []int{0},
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    ticker.Ticker_symbol,
			Subtitle: "Up to last 100 End of Day prices",
		}),
	)

	// Put data into instance
	kline.SetXAxis(x_axis).AddSeries("kline", y_axis).
		SetSeriesOptions(
			charts.WithMarkPointStyleOpts(opts.MarkPointStyle{
				Label: &opts.Label{
					Show: true,
				},
			}),
		)

	return renderToHtml(kline)
}
