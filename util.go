package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func GradeColorCSS(gradeStr string) string {
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

func SinceColorCSS(sinceStr string) string {
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

func PriceMoveColorCSS(amt float32) string {
	if amt > 0 {
		return "text-success"
	}
	if amt < 0 {
		return "text-danger"
	}
	return ""
}

func PriceBigMoveColorCSS(amt float32) string {
	if amt > 5 {
		return "text-success"
	}
	if amt < -5 {
		return "text-danger"
	}
	return ""
}

func PriceMoveIndicatorCSS(amt float32) string {
	if amt > 0 {
		return "text-success fas fa-arrow-up"
	}
	if amt < 0 {
		return "text-danger fas fa-arrow-down"
	}
	return "fa-solid fa-equals"
}

func PriceBigMoveIndicatorCSS(amt float32) string {
	if amt > 5 {
		return "text-warning fas fa-chart-line-up "
	}
	if amt < -5 {
		return "text-warning fas fa-chart-line-down "
	}
	return ""
}

func Concat(strs ...string) string {
	return strings.Trim(strings.Join(strs, ""), " ")
}

func MinutesSince(t time.Time) string {
	return fmt.Sprintf("%.0f min ago", time.Since(t).Minutes())
}

func AttributeColorCSS(attrname, attrvalue string, ticker Ticker) string {
	color := "text-light"

	switch attrname {
	case "Price/Book (Mrq)":
		if value, err := strconv.ParseFloat(attrvalue, 32); err == nil {
			switch {
			case value < 1:
				color = "test-success"
			case value < 3:
				color = "text-warning"
			case value > 1:
				color = "text-danger"
			}
		}
	}

	return color
}
