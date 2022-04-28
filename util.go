package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

func FormatUnixTime(unixTime int64, formatStr string) string {
	if unixTime == 0 {
		return ""
	}
	if formatStr == "" {
		formatStr = "Jan 2 15:04 MST 2006"
	}

	EasternTZ, _ := time.LoadLocation("America/New_York")
	realDate := time.Unix(unixTime, 0).In(EasternTZ)
	return realDate.Format(formatStr)
}

func FormatDatetimeStr(dateStr string, formatStr string) string {
	if formatStr == "" {
		formatStr = "Jan 2"
	}
	var dateObj time.Time
	if len(dateStr) == 10 {
		dateObj, _ = time.Parse("2006-01-02", dateStr)
	} else if len(dateStr) == 19 {
		dateObj, _ = time.Parse(sqlDatetimeSearchType, dateStr)
	} else if len(dateStr) == 20 {
		dateObj, _ = time.Parse("2006-01-02T15:04:05Z", dateStr)
	}

	return dateObj.Format(formatStr)
}

func UnixToDatetimeStr(unixTime int64) string {
	dateTime := time.Unix(unixTime, 0)
	return dateTime.Format(sqlDatetimeSearchType)
}

func GradeColor(gradeStr string) string {
	lcGradeStr := strings.ToLower(gradeStr)
	switch lcGradeStr {
	case "strong buy":
		return "text-success"
	case "buy", "outperform", "moderate buy", "accumulate", "overweight", "add", "market perform", "sector perform":
		return "text-success"
	case "hold", "neutral", "in-line", "equal-weight":
		return "text-warning"
	case "sell", "underperform", "moderate sell", "weak hold", "underweight", "reduce", "market underperform", "sector underperform":
		return "text-danger"
	case "strong sell":
		return "text-danger"
	default:
		return "text-white"
	}
}

func SinceColor(sinceStr string) string {
	lcSinceStr := strings.ToLower(sinceStr)
	up_rx := regexp.MustCompile(`^(and|but) up `)
	down_rx := regexp.MustCompile(`^(and|but) down `)

	if up_rx.MatchString(lcSinceStr) {
		return "text-success"
	} else if down_rx.MatchString(lcSinceStr) {
		return "text-danger"
	} else {
		return "text-white"
	}
}

func isMarketOpen() bool {
	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	timeStr := currentDate.Format("1504")
	weekday := currentDate.Weekday()

	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	if timeStr >= "0930" && timeStr < "1600" {
		return true
	}

	return false
}

func PriceDiffAmt(a, b float64) string {
	return fmt.Sprintf("$%.2f", b-a)
}

func PriceDiffPerc(a, b float64) string {
	return fmt.Sprintf("%.2f%%", (b-a)/a*100)
}

func PriceDiffPercAmt(a, b float64) float64 {
	return (b - a) / a * 100
}
