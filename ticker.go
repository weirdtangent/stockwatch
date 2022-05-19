package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/mytime"
	"github.com/weirdtangent/yhfinance"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Ticker struct {
	TickerId            uint64 `db:"ticker_id"`
	EId                 string
	TickerSymbol        string    `db:"ticker_symbol"`
	TickerType          string    `db:"ticker_type"`
	TickerMarket        string    `db:"ticker_market"`
	ExchangeId          uint64    `db:"exchange_id"`
	TickerName          string    `db:"ticker_name"`
	CompanyName         string    `db:"company_name"`
	Address             string    `db:"address"`
	City                string    `db:"city"`
	State               string    `db:"state"`
	Zip                 string    `db:"zip"`
	Country             string    `db:"country"`
	Website             string    `db:"website"`
	Phone               string    `db:"phone"`
	Sector              string    `db:"sector"`
	Industry            string    `db:"industry"`
	MarketPrice         float64   `db:"market_price"`
	MarketPrevClose     float64   `db:"market_prev_close"`
	MarketVolume        int64     `db:"market_volume"`
	MarketPriceDatetime time.Time `db:"market_price_datetime"`
	FavIconS3Key        string    `db:"favicon_s3key"`
	FetchDatetime       time.Time `db:"fetch_datetime"`
	MSPerformanceId     string    `db:"ms_performance_id"`
	CreateDatetime      time.Time `db:"create_datetime"`
	UpdateDatetime      time.Time `db:"update_datetime"`
}

type TickerAttribute struct {
	TickerAttributeId uint64 `db:"attribute_id"`
	EId               string
	TickerId          uint64 `db:"ticker_id"`
	AttributeName     string `db:"attribute_name"`
	Definition        sql.NullString
	AttributeComment  string    `db:"attribute_comment"`
	AttributeValue    string    `db:"attribute_value"`
	CreateDatetime    time.Time `db:"create_datetime"`
	UpdateDatetime    time.Time `db:"update_datetime"`
}

type TickerDaily struct {
	TickerDailyId  uint64 `db:"ticker_daily_id"`
	EId            string
	TickerId       uint64    `db:"ticker_id"`
	PriceDatetime  time.Time `db:"price_datetime"`
	OpenPrice      float64   `db:"open_price"`
	HighPrice      float64   `db:"high_price"`
	LowPrice       float64   `db:"low_price"`
	ClosePrice     float64   `db:"close_price"`
	Volume         int64     `db:"volume"`
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}

type TickerDailies struct {
	Days []TickerDaily
}

type TickerDescription struct {
	TickerDescriptionId uint64 `db:"description_id"`
	EId                 string
	TickerId            uint64    `db:"ticker_id"`
	BusinessSummary     string    `db:"business_summary"`
	CreateDatetime      time.Time `db:"create_datetime"`
	UpdateDatetime      time.Time `db:"update_datetime"`
}

type TickerIntraday struct {
	TickerIntradayId uint64 `db:"intraday_id"`
	EId              string
	TickerId         uint64    `db:"ticker_id"`
	PriceDate        string    `db:"price_date"`
	LastPrice        float64   `db:"last_price"`
	Volume           int64     `db:"volume"`
	CreateDatetime   time.Time `db:"create_datetime"`
	UpdateDatetime   time.Time `db:"update_datetime"`
}

type TickerIntradays struct {
	Moments []TickerIntraday
}

type TickerUpDown struct {
	TickerUpDownId  uint64 `db:"updown_id"`
	EId             string
	TickerId        uint64       `db:"ticker_id"`
	UpDownAction    string       `db:"updown_action"`
	UpDownFromGrade string       `db:"updown_fromgrade"`
	UpDownToGrade   string       `db:"updown_tograde"`
	UpDownDate      sql.NullTime `db:"updown_date"`
	UpDownFirm      string       `db:"updown_firm"`
	UpDownSince     string       `db:"updown_since"`
	CreateDatetime  time.Time    `db:"create_datetime"`
	UpdateDatetime  time.Time    `db:"update_datetime"`
}

type TickerSplit struct {
	TickerSplitId  uint64 `db:"ticker_split_id"`
	EId            string
	TickerId       uint64    `db:"ticker_id"`
	SplitDate      time.Time `db:"split_date"`
	SplitRatio     string    `db:"split_ratio"`
	CreateDatetime time.Time `db:"create_datetime"`
	UpdateDatetime time.Time `db:"update_datetime"`
}

type TickerQuote struct {
	Ticker      Ticker
	Exchange    Exchange
	Description TickerDescription
	LiveQuote   yhfinance.YHQuote
	LastEOD     TickerDaily
	ChangeDir   string
	ChangeAmt   float32
	ChangePct   float32
	Locked      bool
	FavIcon     string
	SymbolNews  struct {
		LastChecked time.Time
		UpdatingNow bool
		Articles    []WebArticle
	}
}

type TickerDetails struct {
	Attributes []TickerAttribute
	Splits     []TickerSplit
	UpDowns    []TickerUpDown
}

// object methods -------------------------------------------------------------

func (t *Ticker) Update(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db

	var update = "UPDATE ticker SET ticker_type=?, ticker_market=?, exchange_id=?, ticker_name=?, company_name=?, address=?, city=?, state=?, zip=?, country=?, website=?, phone=?, sector=?, industry=?, market_price=?, market_prev_close=?, market_volume=?, market_price_datetime=?, favicon_s3key=?, fetch_datetime=now() WHERE ticker_id=?"
	_, err := db.Exec(update, t.TickerType, t.TickerMarket, t.ExchangeId, t.TickerName, t.CompanyName, t.Address, t.City, t.State, t.Zip, t.Country, t.Website, t.Phone, t.Sector, t.Industry, t.MarketPrice, t.MarketPrevClose, t.MarketVolume, t.MarketPriceDatetime, t.FavIconS3Key, t.TickerId)
	return err
}

func (t *Ticker) UpdateTickerWithLiveQuote(deps *Dependencies, sublog zerolog.Logger, quote yhfinance.YHQuote) error {
	db := deps.db

	t.MarketPrice = quote.QuotePrice
	t.MarketPrevClose = quote.QuotePrevClose
	t.MarketVolume = quote.QuoteVolume
	t.MarketPriceDatetime = time.Unix(quote.QuoteTime, 0)

	var update = "UPDATE ticker SET market_price=?, market_volume=?, market_prev_close=?, market_price_datetime=? WHERE ticker_id=?"
	_, err := db.Exec(update, t.MarketPrice, t.MarketVolume, t.MarketPrevClose, t.MarketPriceDatetime, t.TickerId)
	return err
}

func (t *Ticker) getIdBySymbol(deps *Dependencies, sublog zerolog.Logger) (uint64, error) {
	db := deps.db

	var tickerId uint64
	err := db.QueryRowx("SELECT ticker_id FROM ticker WHERE ticker_symbol=?", t.TickerSymbol).Scan(&tickerId)
	return tickerId, err
}

func (t *Ticker) getById(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_id=?", t.TickerId).StructScan(t)
	return err
}

func (t *Ticker) create(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db

	if t.TickerSymbol == "" {
		// refusing to add ticker with blank symbol
		return nil
	}

	insert := "INSERT INTO ticker SET ticker_symbol=?, ticker_type=?, ticker_market=?, exchange_id=?, ticker_name=?, company_name=?, address=?, city=?, state=?, zip=?, country=?, website=?, phone=?, sector=?, industry=?, market_price=?, market_prev_close=?, market_volume=?, fetch_datetime=?"
	res, err := db.Exec(insert, t.TickerSymbol, t.TickerType, t.TickerMarket, t.ExchangeId, t.TickerName, t.CompanyName, t.Address, t.City, t.State, t.Zip, t.Country, t.Website, t.Phone, t.Sector, t.Industry, t.MarketPrice, t.MarketPrevClose, t.MarketVolume, t.FetchDatetime)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed on INSERT")
		return err
	}
	tickerId, err := res.LastInsertId()
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed on LAST_INSERTID")
		return err
	}
	t.TickerId = uint64(tickerId)
	return nil
}

func (t *Ticker) createOrUpdate(deps *Dependencies, sublog zerolog.Logger) error {
	if t.TickerSymbol == "" {
		sublog.Error().Interface("ticker", t).Msg("refusing to insert ticker with blank symbol")
		return nil
	}

	if t.TickerId == 0 {
		var err error
		t.TickerId, err = t.getIdBySymbol(deps, sublog)
		if errors.Is(err, sql.ErrNoRows) {
			return t.create(deps, sublog)
		}
		if err != nil {
			return err
		}
	}

	t.Update(deps, sublog)
	return t.getById(deps, sublog)
}

func (t *Ticker) createOrUpdateAttribute(deps *Dependencies, sublog zerolog.Logger, attributeName, attributeComment, attributeValue string) error {
	db := deps.db

	attribute := TickerAttribute{0, "", t.TickerId, attributeName, sql.NullString{}, attributeComment, attributeValue, time.Now(), time.Now()}
	err := attribute.getByUniqueKey(deps)
	if err == nil {
		var update = "UPDATE ticker_attribute SET attribute_value=? WHERE ticker_id=? AND attribute_name=? AND attribute_comment=?"
		db.Exec(update, attributeValue, t.TickerId, attributeName, attributeComment)
		return nil
	}

	var insert = "INSERT INTO ticker_attribute SET ticker_id=?, attribute_name=?, attribute_comment=?, attribute_value=?"
	db.Exec(insert, t.TickerId, attributeName, attributeComment, attributeValue)
	return nil
}

// if it is a workday after 4 and we don't have the EOD, or we don't have the prior workday EOD, get them
func (t *Ticker) needEODs(deps *Dependencies, sublog zerolog.Logger) bool {
	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	currentTimeStr := currentDate.Format("1505")
	currentDateStr := currentDate.Format("2006-01-02")
	currentWeekday := currentDate.Weekday()

	todayEOD := t.haveEODForDate(deps, currentDateStr)

	// if it's a workday and the market closed for the day and we don't have today's EOD, then YES
	if currentWeekday != time.Saturday && currentWeekday != time.Sunday && currentTimeStr >= "1600" && !todayEOD {
		return true
	}

	priorWorkDate := mytime.PriorWorkDate(currentDate)
	priorWorkDateStr := priorWorkDate.Format("2006-01-02")

	priorEOD := t.haveEODForDate(deps, priorWorkDateStr)

	return !priorEOD
}

func (t Ticker) haveEODForDate(deps *Dependencies, dateStr string) bool {
	db := deps.db

	// for past days, a time of exactly 9:30:00 is considered a locked-in value
	// but if it is anything else, it needs to be after 16:00:00
	var count int
	err := db.QueryRowx("SELECT COUNT(*) FROM ticker_daily WHERE ticker_id=? AND price_datetime IS BETWEEN ? AND ?", t.TickerId, dateStr+"20:00:00", dateStr+"23:59:59").Scan(&count)
	return err == nil && count > 0
}

func (t Ticker) EarliestEOD(db *sqlx.DB) (string, float64, error) {
	type Earliest struct {
		date  string
		price float64
	}
	var earliest Earliest
	err := db.QueryRowx("SELECT price_datetime, close_price FROM ticker_daily WHERE ticker_id=? ORDER BY price_datetime LIMIT 1", t.TickerId).StructScan(&earliest)
	return earliest.date, earliest.price, err
}

func (t Ticker) getTickerEODs(deps *Dependencies, sublog zerolog.Logger, days int) ([]TickerDaily, error) {
	db := deps.db

	var ticker_daily TickerDaily

	fromDate := mytime.DateStr(days * -1)
	dailies := make([]TickerDaily, 0, days)

	rows, err := db.Queryx(
		`SELECT * FROM (
           SELECT * FROM ticker_daily WHERE ticker_id=? AND volume > 0 AND price_datetime > ?
		   ORDER BY price_datetime DESC) DT1
		 ORDER BY price_datetime`,
		t.TickerId, fromDate)
	if err != nil {
		return dailies, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&ticker_daily)
		if err != nil {
			log.Warn().Err(err).Msg("error reading result rows")
		} else {
			dailies = append(dailies, ticker_daily)
		}
	}
	if err := rows.Err(); err != nil {
		return dailies, err
	}

	return dailies, nil
}

func (t Ticker) getLastTickerEOD(deps *Dependencies, sublog zerolog.Logger) (TickerDaily, error) {
	db := deps.db

	var eod TickerDaily

	err := db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_id=? ORDER BY price_datetime DESC`, t.TickerId).StructScan(&eod)
	if err != nil {
		return TickerDaily{}, err
	}

	return eod, nil
}

func (t Ticker) getUpDowns(deps *Dependencies, sublog zerolog.Logger, daysAgo int) ([]TickerUpDown, error) {
	db := deps.db

	var tickerUpDown TickerUpDown
	upDowns := make([]TickerUpDown, 0)

	rows, err := db.Queryx(
		`SELECT * FROM ticker_updown WHERE ticker_id=? AND TIMESTAMPDIFF(DAY, updown_date, NOW()) < ? ORDER BY updown_date DESC`,
		t.TickerId, daysAgo)
	if err != nil {
		return upDowns, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&tickerUpDown)
		if err != nil {
			sublog.Warn().Err(err).Msg("failed reading result rows")
		} else {
			upDowns = append(upDowns, tickerUpDown)
		}
	}
	if err := rows.Err(); err != nil {
		return upDowns, err
	}

	return upDowns, nil
}

func (t Ticker) getAttributes(deps *Dependencies, sublog zerolog.Logger) ([]TickerAttribute, error) {
	db := deps.db

	var tickerAttribute TickerAttribute
	tickerAttributes := make([]TickerAttribute, 0)

	rows, err := db.Queryx(
		`SELECT ticker_attribute.*,definition.definition FROM ticker_attribute LEFT JOIN definition ON (REPLACE(attribute_name,"_"," ") = term) WHERE ticker_id=?`, t.TickerId)
	if err != nil {
		return tickerAttributes, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&tickerAttribute)
		if err != nil {
			sublog.Warn().Err(err).Msg("error reading result rows")
		} else {
			underscore_rx := regexp.MustCompile(`_`)
			tickerAttribute.AttributeName = string(underscore_rx.ReplaceAll([]byte(tickerAttribute.AttributeName), []byte(" ")))
			tickerAttribute.AttributeName = cases.Title(language.English).String(strings.ToLower(tickerAttribute.AttributeName))
			tickerAttributes = append(tickerAttributes, tickerAttribute)
		}
	}
	if err := rows.Err(); err != nil {
		return tickerAttributes, err
	}

	return tickerAttributes, nil
}

func (t Ticker) getSplits(deps *Dependencies, sublog zerolog.Logger) ([]TickerSplit, error) {
	db := deps.db

	var tickerSplit TickerSplit
	tickerSplits := make([]TickerSplit, 0)

	rows, err := db.Queryx(
		`SELECT * FROM ticker_split WHERE ticker_id=?`, t.TickerId)
	if err != nil {
		return tickerSplits, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&tickerSplit)
		if err != nil {
			log.Warn().Err(err).Msg("error reading result rows")
		} else {
			tickerSplits = append(tickerSplits, tickerSplit)
		}

	}
	if err := rows.Err(); err != nil {
		return tickerSplits, err
	}

	return tickerSplits, nil
}

func (ta *TickerAttribute) getByUniqueKey(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx(`SELECT * FROM ticker_attribute WHERE ticker_id=? AND attribute_name=?`, ta.TickerId, ta.AttributeName).StructScan(ta)
	return err
}

type ByTickerPriceDate TickerDailies

func (a ByTickerPriceDate) Len() int { return len(a.Days) }
func (a ByTickerPriceDate) Less(i, j int) bool {
	return a.Days[i].PriceDatetime.Before(a.Days[j].PriceDatetime)
}
func (a ByTickerPriceDate) Swap(i, j int) { a.Days[i], a.Days[j] = a.Days[j], a.Days[i] }

func (td TickerDailies) Sort() *TickerDailies {
	sort.Sort(ByTickerPriceDate(td))
	return &td
}

func (td TickerDailies) Reverse() *TickerDailies {
	sort.Sort(sort.Reverse(ByTickerPriceDate(td)))
	return &td
}

func (td TickerDailies) Count() int {
	return len(td.Days)
}

func (td *TickerDaily) IsFinalPrice() bool {
	return td.PriceDatetime.Format("15:04:05") == "09:30:00" || td.PriceDatetime.Format("15:04:05") >= "16:00:00"
}

func (td *TickerDaily) checkByDate(deps *Dependencies) uint64 {
	db := deps.db

	var tickerDailyId uint64
	db.QueryRowx(`SELECT ticker_daily_id FROM ticker_daily WHERE ticker_id=? AND price_datetime=?`, td.TickerId, td.PriceDatetime).Scan(&tickerDailyId)
	return tickerDailyId
}

func (td *TickerDaily) create(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db
	_, calling_file, calling_line, _ := runtime.Caller(1)
	tasklog := sublog.With().Str("called_by", fmt.Sprintf("%s %d", calling_file, calling_line)).Logger()

	if td.Volume == 0 {
		// Refusing to add ticker daily with 0 volume
		return nil
	}

	var insert = "INSERT INTO ticker_daily SET ticker_id=?, price_datetime=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=?"
	_, err := db.Exec(insert, td.TickerId, td.PriceDatetime, td.OpenPrice, td.HighPrice, td.LowPrice, td.ClosePrice, td.Volume)
	if err != nil {
		tasklog.Fatal().Err(err).Msg("failed on INSERT")
	}
	return err
}

func (td *TickerDaily) createOrUpdate(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db

	if td.Volume == 0 {
		// Refusing to add ticker daily with 0 volume
		return nil
	}

	td.TickerDailyId = td.checkByDate(deps)
	if td.TickerDailyId == 0 {
		return td.create(deps, sublog)
	}

	var update = "UPDATE ticker_daily SET price_datetime=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=? WHERE ticker_id=? AND price_datetime=?"
	_, err := db.Exec(update, td.PriceDatetime, td.OpenPrice, td.HighPrice, td.LowPrice, td.ClosePrice, td.Volume, td.TickerId, td.PriceDatetime)
	if err != nil {
		sublog.Warn().Err(err).Msg("failed on UPDATE")
	}
	return err
}

func (td *TickerDescription) getByUniqueKey(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx(`SELECT * FROM ticker_description WHERE ticker_id=?`, td.TickerId).StructScan(td)
	return err
}

func (td *TickerDescription) createOrUpdate(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db

	if td.BusinessSummary == "" {
		return nil
	}

	newBusinessSummary := td.BusinessSummary
	err := td.getByUniqueKey(deps)
	if err == nil {
		update := "UPDATE ticker_description SET business_summary=? WHERE description_id=?"
		_, err = db.Exec(update, newBusinessSummary, td.TickerDescriptionId)
		if err != nil {
			sublog.Fatal().Err(err).Msg("failed on update")
		}
		return err
	}

	var insert = "INSERT INTO ticker_description SET ticker_id=?, business_summary=?"
	_, err = db.Exec(insert, td.TickerId, td.BusinessSummary)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed on insert")
	}
	return err
}

type ByTickerPriceTime TickerIntradays

func (a ByTickerPriceTime) Len() int { return len(a.Moments) }
func (a ByTickerPriceTime) Less(i, j int) bool {
	return a.Moments[i].PriceDate < a.Moments[j].PriceDate
}
func (a ByTickerPriceTime) Swap(i, j int) { a.Moments[i], a.Moments[j] = a.Moments[j], a.Moments[i] }

func (i TickerIntradays) Sort() *TickerIntradays {
	sort.Sort(ByTickerPriceTime(i))
	return &i
}

func (i TickerIntradays) Reverse() *TickerIntradays {
	sort.Sort(sort.Reverse(ByTickerPriceTime(i)))
	return &i
}

func (i TickerIntradays) Count() int {
	return len(i.Moments)
}

func (tud *TickerUpDown) getByUniqueKey(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx(`SELECT * FROM ticker_updown WHERE ticker_id=? AND updown_date=? AND updown_firm=?`, tud.TickerId, tud.UpDownDate, tud.UpDownFirm).StructScan(tud)
	return err
}

func (tud *TickerUpDown) createIfNew(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db

	if tud.UpDownToGrade == "" {
		return nil
	}

	// if already exists, just quietly return
	err := tud.getByUniqueKey(deps)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_updown SET ticker_id=?, updown_action=?, updown_fromgrade=?, updown_tograde=?, updown_date=?, updown_firm=?"
	_, err = db.Exec(insert, tud.TickerId, tud.UpDownAction, tud.UpDownFromGrade, tud.UpDownToGrade, tud.UpDownDate, tud.UpDownFirm)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed on INSERT")
	}
	return err
}

func (ts *TickerSplit) getByDate(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx(`SELECT * FROM ticker_split WHERE ticker_id=? AND split_date=?`, ts.TickerId, ts.SplitDate).StructScan(ts)
	return err
}

func (ts *TickerSplit) createIfNew(deps *Dependencies, sublog zerolog.Logger) error {
	db := deps.db

	if ts.SplitRatio == "" {
		// Refusing to add ticker split with blank ratio
		return nil
	}

	err := ts.getByDate(deps)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_split SET ticker_id=?, split_date=?, split_ratio=?"
	_, err = db.Exec(insert, ts.TickerId, ts.SplitDate, ts.SplitRatio)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed on INSERT")
	}
	return err
}

func (t *Ticker) GetFinancials(deps *Dependencies, sublog zerolog.Logger, period, chartType string, isPercentage int) ([]string, []map[string]float64, error) {
	db := deps.db

	var periodStrs = []string{}
	var barValues = []map[string]float64{}

	rows, err := db.Queryx(`SELECT chart_datetime,
	                          group_concat(chart_name) AS chart_names,
	                          group_concat(chart_value) AS chart_values
						    FROM (SELECT * FROM financials WHERE ticker_id=? AND form_term_name=? AND chart_type=? AND is_percentage=? ORDER BY chart_datetime) t
							GROUP BY 1`, t.TickerId, period, chartType, isPercentage)
	if err != nil {
		return periodStrs, barValues, err
	}
	defer rows.Close()

	var financials struct {
		ChartDatetime sql.NullTime `db:"chart_datetime"`
		ChartNames    string       `db:"chart_names"`
		ChartValues   string       `db:"chart_values"`
	}
	for rows.Next() {
		err = rows.StructScan(&financials)
		if err != nil {
			log.Warn().Err(err).
				Str("table_name", "financials").
				Msg("error reading result rows")
		} else {
			var barTime string
			if period == "Quarterly" {
				barTime = financials.ChartDatetime.Time.Format("2006-01")
			} else {
				barTime = financials.ChartDatetime.Time.Format("2006")
			}

			periodStrs = append(periodStrs, barTime)
			categories := strings.Split(financials.ChartNames, ",")
			values := map[string]float64{}
			for x, strValue := range strings.Split(financials.ChartValues, ",") {
				values[categories[x]], _ = strconv.ParseFloat(strValue, 64)
			}
			barValues = append(barValues, values)
		}
	}
	if err := rows.Err(); err != nil {
		return periodStrs, barValues, err
	}

	return periodStrs, barValues, nil
}

func (t Ticker) queueSaveFavIcon(deps *Dependencies, sublog zerolog.Logger) error {
	awssess := deps.awssess
	awssvc := sqs.New(awssess)
	queueName := "stockwatch-tickers"

	urlResult, err := awssvc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		return err
	}

	type TaskTickerNewsBody struct {
		TickerId     uint64 `json:"ticker_id"`
		EId          string
		TickerSymbol string `json:"ticker_symbol"`
		ExchangeId   uint64 `json:"exchange_id"`
	}

	// get next message from queue, if any
	queueURL := urlResult.QueueUrl
	messageBytes, _ := json.Marshal(TaskTickerNewsBody{TickerSymbol: t.TickerSymbol})
	messageBody := string(messageBytes)
	messageAttributes := map[string]*sqs.MessageAttributeValue{
		"action": {
			DataType:    aws.String("String"),
			StringValue: aws.String("favicon"),
		}}
	_, err = awssvc.SendMessage(&sqs.SendMessageInput{
		MessageBody:       aws.String(messageBody),
		MessageAttributes: messageAttributes,
		QueueUrl:          queueURL,
	})
	return err
}

func (t *Ticker) getFavIconCDATA(deps *Dependencies, sublog zerolog.Logger) string {
	awssess := deps.awssess

	if t.FavIconS3Key == "none" {
		return ""
	}
	if t.FavIconS3Key == "" {
		t.queueSaveFavIcon(deps, sublog)
		return ""
	}
	redisPool := deps.redisPool

	redisConn := redisPool.Get()
	defer redisConn.Close()

	// pull URL from redis (1 day expire), or go get from AWS
	redisKey := "aws/s3/" + t.FavIconS3Key
	data, err := redis.String(redisConn.Do("GET", redisKey))
	if err == nil && !skipRedisChecks {
		return data
	}

	s3svc := s3.New(awssess)

	inputGetObj := &s3.GetObjectInput{
		Bucket: aws.String(awsPrivateBucketName),
		Key:    aws.String(t.FavIconS3Key),
	}
	resp, err := s3svc.GetObject(inputGetObj)
	if err != nil {
		sublog.Info().Msg(fmt.Sprintf("%#v", err))
		sublog.Error().Err(err).Str("symbol", t.TickerSymbol).Str("s3key", t.FavIconS3Key).Msg("failed to get s3 object from aws")
		return ""
	}
	defer resp.Body.Close()

	size := resp.ContentLength
	buffer := make([]byte, int(*size))
	var bbuffer bytes.Buffer
	for {
		num, rerr := resp.Body.Read(buffer)
		if num > 0 {
			bbuffer.Write(buffer[:num])
		} else if rerr == io.EOF || rerr != nil {
			break
		}
	}

	data = base64.StdEncoding.EncodeToString(bbuffer.Bytes())

	_, err = redisConn.Do("SET", redisKey, data, "EX", 60*60*24)
	if err != nil {
		sublog.Error().Err(err).Str("symbol", t.TickerSymbol).Str("redis_key", redisKey).Msg("failed to save to redis")
	}

	return data
}

// misc -----------------------------------------------------------------------

func getTickerBySymbol(deps *Dependencies, sublog zerolog.Logger, symbol string) (Ticker, error) {
	db := deps.db

	ticker := Ticker{}
	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=?", symbol).StructScan(&ticker)
	if err == nil {
		ticker.EId = encryptId(deps, *deps.logger, "ticker", ticker.TickerId)
	}
	return ticker, err
}

func getTickerDescriptionById(deps *Dependencies, sublog zerolog.Logger, ticker_id uint64) (TickerDescription, error) {
	db := deps.db

	var tickerDescription TickerDescription
	err := db.QueryRowx(`SELECT * FROM ticker_description WHERE ticker_id=?`, ticker_id).StructScan(&tickerDescription)
	if err == nil {
		tickerDescription.EId = encryptId(deps, *deps.logger, "ticker_description", tickerDescription.TickerDescriptionId)
	}
	return tickerDescription, err
}

func getTickerQuote(deps *Dependencies, sublog zerolog.Logger, watcher Watcher, symbol string) (TickerQuote, error) {
	start := time.Now()
	tickerQuote := TickerQuote{}
	sublog = sublog.With().Str("watcher", watcher.EId).Str("symbol", symbol).Logger()

	ticker, err := getFreshTicker(deps, sublog, symbol)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to getFreshTicker")
		return TickerQuote{}, err
	}
	tickerQuote.Ticker = ticker

	exchange, err := getExchangeById(deps, sublog, ticker.ExchangeId)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to getExchangeById")
		return TickerQuote{}, err
	}
	tickerQuote.Exchange = exchange

	tickerDescription, err := getTickerDescriptionById(deps, sublog, ticker.TickerId)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to getTickerDescriptionById")
		return TickerQuote{}, err
	}
	tickerQuote.Description = tickerDescription

	// if the market is open, lets get a live quote
	if isMarketOpen() {
		quote, err := fetchTickerQuoteFromYH(deps, sublog, ticker)
		if err == nil {
			tickerQuote.LiveQuote = quote
			tickerQuote.Ticker.UpdateTickerWithLiveQuote(deps, sublog, quote)
		}
	} else {
		if tickerQuote.Ticker.needEODs(deps, sublog) {
			err := fetchTickerEODsFromYH(deps, sublog, ticker)
			if err != nil {
				sublog.Error().Err(err).Msg("failed to fetch tickerEODs")
				return TickerQuote{}, err
			}
		}
		lastEOD, err := tickerQuote.Ticker.getLastTickerEOD(deps, sublog)
		if err == nil {
			tickerQuote.LastEOD = lastEOD
		}
	}

	if ticker.MarketPrice > 0 && ticker.MarketPrevClose > 0 {
		tickerQuote.ChangeAmt = float32(ticker.MarketPrice - ticker.MarketPrevClose)
		tickerQuote.ChangePct = float32((ticker.MarketPrice - ticker.MarketPrevClose) / ticker.MarketPrevClose * 100)
		if ticker.MarketPrice-ticker.MarketPrevClose > 0 {
			tickerQuote.ChangeDir = "up"
		} else if ticker.MarketPrice-ticker.MarketPrevClose < 0 {
			tickerQuote.ChangeDir = "down"
		} else {
			tickerQuote.ChangeDir = "unchanged"
		}
	}

	tickerQuote.Locked, err = isWatcherRecent(deps, sublog, watcher, ticker)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to check isWatcherRecent")
		return TickerQuote{}, err
	}

	articles, err := getArticlesByTicker(deps, sublog, ticker, 20, time.Duration(180*24*time.Hour))
	if err != nil {
		sublog.Error().Err(err).Msg("failed to getArticlesByTicker")
		return TickerQuote{}, err
	}
	tickerQuote.SymbolNews.Articles = articles

	// schedule to update ticker news
	lastdone := LastDone{Activity: "ticker_news", UniqueKey: ticker.TickerSymbol}
	err = lastdone.getByActivity(deps)
	if err == nil && lastdone.LastStatus == "success" {
		tickerQuote.SymbolNews.LastChecked = lastdone.LastDoneDatetime.Time
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerNewsDelay).Before(time.Now()) {
			err = ticker.queueUpdateNews(deps)
			if err != nil {
				sublog.Error().Err(err).Msg("failed to queue UpdateNews")
			}
			tickerQuote.SymbolNews.UpdatingNow = true
		}
	} else {
		err = ticker.queueUpdateNews(deps)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to queue UpdateNews")
		}
		tickerQuote.SymbolNews.UpdatingNow = true
	}

	// schedule to update ticker financials
	lastdone = LastDone{Activity: "ticker_financials", UniqueKey: ticker.TickerSymbol}
	err = lastdone.getByActivity(deps)
	if err == nil && lastdone.LastStatus == "success" {
		if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerFinancialsDelay).Before(time.Now()) {
			err = ticker.queueUpdateFinancials(deps)
			if err != nil {
				sublog.Error().Err(err).Msg("failed to queue UpdateFinancials")
			}
		}
	} else {
		err = ticker.queueUpdateFinancials(deps)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to queue UpdateFinancials")
		}
	}

	tickerQuote.FavIcon = ticker.getFavIconCDATA(deps, sublog)

	sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Msg("timer: getTickerQuote")
	return tickerQuote, err
}

func getFreshTicker(deps *Dependencies, sublog zerolog.Logger, symbol string) (Ticker, error) {
	// load ticker from DB or go get from YH if new
	// also, update from YH if ticker is over 24 hours old
	ticker, err := getTickerBySymbol(deps, sublog, symbol)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		ticker, err = fetchTickerInfoFromYH(deps, sublog, symbol)
		if err != nil {
			return Ticker{}, err
		}
		ticker.FetchDatetime = time.Now()
		err = ticker.Update(deps, sublog)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to save updates to ticker")
		}
	} else if err != nil {
		return Ticker{}, err
	} else if ticker.FetchDatetime.Before(time.Now().Add(-24*time.Hour)) ||
		(!isMarketOpen() && time.Since(ticker.FetchDatetime).Minutes() > minTickerReloadDelayOpen) ||
		(isMarketOpen() && time.Since(ticker.FetchDatetime).Minutes() > minTickerReloadDelayClosed) {
		ticker, err = fetchTickerInfoFromYH(deps, sublog, symbol)
		if err != nil {
			return Ticker{}, err
		}
		ticker.FetchDatetime = time.Now()
		err = ticker.Update(deps, sublog)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to save updates to ticker")
		}
	}
	return ticker, nil
}

func getFreshTickers(deps *Dependencies, sublog zerolog.Logger, symbols []string) ([]Ticker, error) {
	tickers := []Ticker{}

	quotes, err := loadMultiTickerQuotes(deps, sublog, symbols)
	if err != nil {
		return []Ticker{}, err
	}

	for symbol, quote := range quotes {
		ticker, err := getTickerBySymbol(deps, sublog, symbol)
		if err != nil {
			return []Ticker{}, err
		}

		ticker.FetchDatetime = time.Now()
		ticker.MarketPrice = quote.QuotePrice
		ticker.MarketVolume = quote.QuoteVolume
		ticker.MarketPriceDatetime = time.Unix(quote.QuoteTime, 0)
		err = ticker.Update(deps, sublog)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to save updates to ticker")
		}
		tickers = append(tickers, ticker)
	}

	return tickers, nil
}

func getTickerDetails(deps *Dependencies, sublog zerolog.Logger, watcher Watcher, symbol string) (TickerDetails, error) {
	tickerDetails := TickerDetails{}

	ticker, err := getFreshTicker(deps, sublog, symbol)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to getFreshTicker")
		return TickerDetails{}, err
	}

	tickerAttributes, err := ticker.getAttributes(deps, sublog)
	if err != nil {
		return TickerDetails{}, err
	}
	tickerDetails.Attributes = tickerAttributes

	tickerSplits, err := ticker.getSplits(deps, sublog)
	if err != nil {
		return TickerDetails{}, err
	}
	tickerDetails.Splits = tickerSplits

	tickerUpDowns, err := ticker.getUpDowns(deps, sublog, 90)
	if err != nil {
		return TickerDetails{}, err
	}
	tickerDetails.UpDowns = tickerUpDowns

	return tickerDetails, nil
}

func getRecentsQuotes(deps *Dependencies, sublog zerolog.Logger, watcher Watcher, recents []WatcherRecent) ([]TickerQuote, error) {
	start := time.Now()
	tickerQuotes := []TickerQuote{}

	symbols := []string{}
	for _, recent := range recents {
		symbols = append(symbols, recent.TickerSymbol)
	}

	tickers, err := getFreshTickers(deps, sublog, symbols)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to getFreshTicker")
		return []TickerQuote{}, err
	}
	for _, ticker := range tickers {
		symbol := ticker.TickerSymbol
		tickerQuote := TickerQuote{}
		tickerQuote.Ticker = ticker

		exchange, err := getExchangeById(deps, sublog, ticker.ExchangeId)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to getExchangeById")
			return []TickerQuote{}, err
		}
		tickerQuote.Exchange = exchange

		tickerDescription, err := getTickerDescriptionById(deps, sublog, ticker.TickerId)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to getTickerDescriptionById")
			return []TickerQuote{}, err
		}
		tickerQuote.Description = tickerDescription

		if ticker.MarketPrice > 0 && ticker.MarketPrevClose > 0 {
			tickerQuote.ChangeAmt = float32(ticker.MarketPrice - ticker.MarketPrevClose)
			tickerQuote.ChangePct = float32((ticker.MarketPrice - ticker.MarketPrevClose) / ticker.MarketPrevClose * 100)
		}

		tickerQuote.Locked, err = isWatcherRecent(deps, sublog, watcher, ticker)
		if err != nil {
			sublog.Error().Err(err).Msg("failed to check isWatcherRecent")
			return []TickerQuote{}, err
		}

		articles, err := getArticlesByTicker(deps, sublog, ticker, 5, time.Duration(7*24*time.Hour))
		if err != nil {
			sublog.Error().Err(err).Msg("failed to getArticlesByTicker")
			return []TickerQuote{}, err
		}
		tickerQuote.SymbolNews.Articles = articles

		if !isMarketOpen() {
			lastEOD, err := tickerQuote.Ticker.getLastTickerEOD(deps, sublog)
			if err == nil {
				tickerQuote.LastEOD = lastEOD
			}
		}

		// schedule to update ticker news
		lastdone := LastDone{Activity: "ticker_news", UniqueKey: ticker.TickerSymbol}
		err = lastdone.getByActivity(deps)
		if err == nil && lastdone.LastStatus == "success" {
			tickerQuote.SymbolNews.LastChecked = lastdone.LastDoneDatetime.Time
			if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerNewsDelay).Before(time.Now()) {
				err = ticker.queueUpdateNews(deps)
				if err != nil {
					sublog.Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateNews")
				}
				tickerQuote.SymbolNews.UpdatingNow = true
			}
		} else {
			err = ticker.queueUpdateNews(deps)
			if err != nil {
				sublog.Error().Err(err).Str("ticker", symbol).Uint64("exchange_id", ticker.ExchangeId).Msg("failed to queue UpdateNews")
			}
			tickerQuote.SymbolNews.UpdatingNow = true
		}

		// schedule to update ticker financials
		lastdone = LastDone{Activity: "ticker_financials", UniqueKey: ticker.TickerSymbol}
		err = lastdone.getByActivity(deps)
		if err == nil && lastdone.LastStatus == "success" {
			if lastdone.LastDoneDatetime.Time.Add(time.Minute * minTickerFinancialsDelay).Before(time.Now()) {
				err = ticker.queueUpdateFinancials(deps)
				if err != nil {
					sublog.Error().Err(err).Msg("failed to queue UpdateFinancials")
				}
			}
		} else {
			err = ticker.queueUpdateFinancials(deps)
			if err != nil {
				sublog.Error().Err(err).Msg("failed to queue UpdateFinancials")
			}
		}

		tickerQuote.FavIcon = ticker.getFavIconCDATA(deps, sublog)
		tickerQuotes = append(tickerQuotes, tickerQuote)
	}

	sublog.Info().Int64("response_time", time.Since(start).Nanoseconds()).Msg("timer: getRecentsQuotes")
	return tickerQuotes, err
}
