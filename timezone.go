package main

import (
	"io/ioutil"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type Timezone struct {
	Location string
	Default  bool
	TZAbbr   string
	Offset   string
	Text     string
}

func getTimezones(deps *Dependencies, sublog zerolog.Logger) []Timezone {
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
