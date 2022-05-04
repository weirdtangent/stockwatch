package main

import (
	"io/ioutil"
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

func PriceDiffAmt(a, b float64) float64 {
	return b - a
}

func PriceDiffPercAmt(a, b float64) float64 {
	return (b - a) / a * 100
}

func PriceMoveColorCSS(amt float64) string {
	if amt > 0 {
		return "text-success"
	}
	if amt < 0 {
		return "text-danger"
	}
	return ""
}

func PriceBigMoveColorCSS(amt float64) string {
	if amt > 5 {
		return "text-success"
	}
	if amt < -5 {
		return "text-danger"
	}
	return ""
}

func PriceMoveIndicatorCSS(amt float64) string {
	if amt > 0 {
		return "text-success fas fa-arrow-up"
	}
	if amt < 0 {
		return "text-danger fas fa-arrow-down"
	}
	return "fa-solid fa-equals"
}

func PriceBigMoveIndicatorCSS(amt float64) string {
	if amt > 5 {
		return "text-warning fas fa-chart-line-up "
	}
	if amt < -5 {
		return "text-warning fas fa-chart-line-down "
	}
	return ""
}

type Timezone struct {
	Location string
	Default  bool
	TZAbbr   string
	Offset   string
	Text     string
}

func getTimezones(deps *Dependencies) []Timezone {
	var tzlist []Timezone

	tzlist = append(tzlist, procTimezoneDir(deps, zoneDir, "")...)
	return tzlist
}

func procTimezoneDir(deps *Dependencies, zoneDir, path string) []Timezone {
	var timezones []Timezone

	watcherTZ := "UTC"
	// if webdata["TZLocation"] != nil {
	// 	watcherTZ = webdata["TZLocation"].(string)
	// }

	files, err := ioutil.ReadDir(zoneDir + path)
	if err != nil {
		return []Timezone{}
	}
	for _, f := range files {
		if f.Name() != strings.ToUpper(f.Name()[:1])+f.Name()[1:] {
			continue
		}
		if f.IsDir() {
			timezones = append(timezones, procTimezoneDir(deps, zoneDir, path+"/"+f.Name())...)
		} else {
			tzfile := (path + "/" + f.Name())[1:]
			tz, err := time.LoadLocation(tzfile)
			if err == nil {
				now := time.Now().In(tz).Format("MST -0700")
				tzabbr := now[:4]
				offset := now[4:]
				if tzabbr[:3] == offset[:3] {
					tzabbr = ""
				}
				timezones = append(timezones, Timezone{
					Location: tz.String(),
					Default:  tz.String() == watcherTZ,
					TZAbbr:   tzabbr,
					Offset:   offset,
					Text:     tz.String() + " [" + now + "]",
				})
			}
		}
	}
	return timezones
}

func TimeNow(loc string) string {
	if loc == "" {
		loc = "UTC"
	}
	tzloc, err := time.LoadLocation(loc)
	if err != nil {
		tzloc, _ = time.LoadLocation("UTC")
	}
	return time.Now().In(tzloc).Format(fullDatetime)
}

func Concat(strs ...string) string {
	return strings.Trim(strings.Join(strs, ""), " ")
}
