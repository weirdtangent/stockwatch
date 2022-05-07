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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/mytime"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Ticker struct {
	TickerId        uint64 `db:"ticker_id"`
	EId             string
	TickerSymbol    string       `db:"ticker_symbol"`
	ExchangeId      uint64       `db:"exchange_id"`
	TickerName      string       `db:"ticker_name"`
	CompanyName     string       `db:"company_name"`
	Address         string       `db:"address"`
	City            string       `db:"city"`
	State           string       `db:"state"`
	Zip             string       `db:"zip"`
	Country         string       `db:"country"`
	Website         string       `db:"website"`
	Phone           string       `db:"phone"`
	Sector          string       `db:"sector"`
	Industry        string       `db:"industry"`
	FavIconS3Key    string       `db:"favicon_s3key"`
	FetchDatetime   sql.NullTime `db:"fetch_datetime"`
	MSPerformanceId string       `db:"ms_performance_id"`
	CreateDatetime  time.Time    `db:"create_datetime"`
	UpdateDatetime  time.Time    `db:"update_datetime"`
}

type TickersEODTask struct {
	TaskAction string `json:"action"`
	TickerId   uint64 `json:"ticker_id"`
	DaysBack   int    `json:"days_back"`
	Offset     int    `json:"offset"`
}

type TickerAttribute struct {
	TickerAttributeId uint64 `db:"attribute_id"`
	EId               string
	TickerId          uint64    `db:"ticker_id"`
	AttributeName     string    `db:"attribute_name"`
	AttributeComment  string    `db:"attribute_comment"`
	AttributeValue    string    `db:"attribute_value"`
	CreateDatetime    time.Time `db:"create_datetime"`
	UpdateDatetime    time.Time `db:"update_datetime"`
}

type TickerDaily struct {
	TickerDailyId  uint64 `db:"ticker_daily_id"`
	EId            string
	TickerId       uint64    `db:"ticker_id"`
	PriceDate      string    `db:"price_date"`
	PriceTime      string    `db:"price_time"`
	PriceDatetime  time.Time `db:"price_datetime"`
	OpenPrice      float64   `db:"open_price"`
	HighPrice      float64   `db:"high_price"`
	LowPrice       float64   `db:"low_price"`
	ClosePrice     float64   `db:"close_price"`
	Volume         float64   `db:"volume"`
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
	Volume           float64   `db:"volume"`
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

func (t *Ticker) getBySymbol(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=?", t.TickerSymbol).StructScan(t)
	return err
}

func (t *Ticker) getIdBySymbol(deps *Dependencies) (uint64, error) {
	db := deps.db

	var tickerId uint64
	err := db.QueryRowx("SELECT ticker_id FROM ticker WHERE ticker_symbol=?", t.TickerSymbol).Scan(&tickerId)
	return tickerId, err
}

func (t *Ticker) getById(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_id=?", t.TickerId).StructScan(t)
	return err
}

func (t *Ticker) create(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

	if t.TickerSymbol == "" {
		// refusing to add ticker with blank symbol
		return nil
	}

	var insert = "INSERT INTO ticker SET ticker_symbol=?, exchange_id=?, ticker_name=?, company_name=?, address=?, city=?, state=?, zip=?, country=?, website=?, phone=?, sector=?, industry=?"
	if !t.FetchDatetime.Valid {
		insert += ", fetch_datetime=now()"
	}
	res, err := db.Exec(insert, t.TickerSymbol, t.ExchangeId, t.TickerName, t.CompanyName, t.Address, t.City, t.State, t.Zip, t.Country, t.Website, t.Phone, t.Sector, t.Industry)
	if err != nil {
		sublog.Fatal().Err(err).Str("ticker", t.TickerSymbol).Msg("failed on INSERT")
		return err
	}
	tickerId, err := res.LastInsertId()
	if err != nil {
		sublog.Fatal().Err(err).Str("symbol", t.TickerSymbol).Msg("failed on LAST_INSERTID")
		return err
	}
	t.TickerId = uint64(tickerId)
	return nil
}

func (t *Ticker) createOrUpdate(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

	if t.TickerSymbol == "" {
		// refusing to add ticker with blank symbol
		return nil
	}

	if t.TickerId == 0 {
		var err error
		t.TickerId, err = t.getIdBySymbol(deps)
		if errors.Is(err, sql.ErrNoRows) || t.TickerId == 0 {
			return t.create(deps)
		}
	}

	var update = "UPDATE ticker SET exchange_id=?, ticker_name=?, company_name=?, address=?, city=?, state=?, zip=?, country=?, website=?, phone=?, sector=?, industry=?, favicon_s3key=?, fetch_datetime=now() WHERE ticker_id=?"
	_, err := db.Exec(update, t.ExchangeId, t.TickerName, t.CompanyName, t.Address, t.City, t.State, t.Zip, t.Country, t.Website, t.Phone, t.Sector, t.Industry, t.FavIconS3Key, t.TickerId)
	if err != nil {
		sublog.Error().Err(err).Str("symbol", t.TickerSymbol).Msg("failed on update")
	}
	return t.getById(deps)
}

func (t *Ticker) createOrUpdateAttribute(deps *Dependencies, attributeName, attributeComment, attributeValue string) error {
	db := deps.db

	attribute := TickerAttribute{0, "", t.TickerId, attributeName, attributeComment, attributeValue, time.Now(), time.Now()}
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
func (t *Ticker) needEODs(deps *Dependencies) bool {
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
	err := db.QueryRowx("SELECT COUNT(*) FROM ticker_daily WHERE ticker_id=? AND price_date=? AND (price_time = ? OR price_time >= ?)", t.TickerId, dateStr, "09:30:00", "16:00:00").Scan(&count)
	return err == nil && count > 0
}

func (t Ticker) EarliestEOD(db *sqlx.DB) (string, float64, error) {
	type Earliest struct {
		date  string
		price float64
	}
	var earliest Earliest
	err := db.QueryRowx("SELECT price_date, close_price FROM ticker_daily WHERE ticker_id=? ORDER BY price_date LIMIT 1", t.TickerId).StructScan(&earliest)
	return earliest.date, earliest.price, err
}

func (t Ticker) ScheduleEODUpdate(deps *Dependencies) bool {
	db := deps.db
	sublog := deps.logger

	earliest, _, err := t.EarliestEOD(db)
	if err != nil {
		sublog.Error().Err(err).Msg("failed to find earliest EOD")
	}

	if len(earliest) > 0 {
		if days, err := mytime.DaysAgo(earliest); err != nil || days > 900 {
			return false
		}
	}

	taskAction := "eod"

	// submit task for 1000 EODs
	taskVars := TickersEODTask{taskAction, t.TickerId, 1000, 0}
	taskJSON, err := json.Marshal(taskVars)
	sublog.Info().Msg(string(taskJSON))
	if err != nil {
		sublog.Error().Err(err).
			Uint64("ticker_id", t.TickerId).
			Msg("failed to create task JSON for EOD update for ticker")
		return false
	}

	_, err = sendNotification(deps, "tickers", taskAction, string(taskJSON))
	if err == nil {
		sublog.Info().
			Uint64("ticker_id", t.TickerId).
			Msg("sent task notification for EOD update for ticker")
	}

	// submit task for 1000 more EODs
	taskVars = TickersEODTask{taskAction, t.TickerId, 1000, 1000}
	taskJSON, err = json.Marshal(taskVars)
	sublog.Info().Msg(string(taskJSON))
	if err != nil {
		sublog.Error().Err(err).
			Uint64("ticker_id", t.TickerId).
			Msg("failed to create task JSON for EOD update for ticker")
		return true
	}

	_, err = sendNotification(deps, "tickers", taskAction, string(taskJSON))
	if err == nil {
		sublog.Info().
			Uint64("ticker_id", t.TickerId).
			Msg("sent task notification for EOD update for ticker")
	}
	return true
}

func (t Ticker) getTickerEODs(deps *Dependencies, days int) ([]TickerDaily, error) {
	db := deps.db

	var ticker_daily TickerDaily

	fromDate := mytime.DateStr(days * -1)
	dailies := make([]TickerDaily, 0, days)

	rows, err := db.Queryx(
		`SELECT * FROM (
           SELECT * FROM ticker_daily WHERE ticker_id=? AND volume > 0 AND price_date > ?
		   ORDER BY price_date DESC) DT1
		 ORDER BY price_date`,
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
			ticker_daily.PriceDatetime, _ = time.Parse(sqlDatetimeParseType, ticker_daily.PriceDate[:11]+ticker_daily.PriceTime+"Z")
			dailies = append(dailies, ticker_daily)
		}
	}
	if err := rows.Err(); err != nil {
		return dailies, err
	}

	return dailies, nil
}

func (t Ticker) getUpDowns(deps *Dependencies, daysAgo int) ([]TickerUpDown, error) {
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
			log.Warn().Err(err).Msg("error reading result rows")
		} else {
			upDowns = append(upDowns, tickerUpDown)
		}
	}
	if err := rows.Err(); err != nil {
		return upDowns, err
	}

	return upDowns, nil
}

func (t Ticker) getAttributes(deps *Dependencies) ([]TickerAttribute, error) {
	db := deps.db

	var tickerAttribute TickerAttribute
	tickerAttributes := make([]TickerAttribute, 0)

	rows, err := db.Queryx(
		`SELECT * FROM ticker_attribute WHERE ticker_id=?`, t.TickerId)
	if err != nil {
		return tickerAttributes, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&tickerAttribute)
		if err != nil {
			log.Warn().Err(err).Msg("error reading result rows")
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

func (t Ticker) getSplits(deps *Dependencies) ([]TickerSplit, error) {
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

func (t Ticker) queueUpdateInfo(deps *Dependencies) error {
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
			StringValue: aws.String("info"),
		}}
	_, err = awssvc.SendMessage(&sqs.SendMessageInput{
		MessageBody:       aws.String(messageBody),
		MessageAttributes: messageAttributes,
		QueueUrl:          queueURL,
	})
	return err
}

func (t Ticker) queueUpdateNews(deps *Dependencies) error {
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
			StringValue: aws.String("news"),
		}}
	_, err = awssvc.SendMessage(&sqs.SendMessageInput{
		MessageBody:       aws.String(messageBody),
		MessageAttributes: messageAttributes,
		QueueUrl:          queueURL,
	})
	return err
}

func (t Ticker) queueUpdateFinancials(deps *Dependencies) error {
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
			StringValue: aws.String("financials"),
		}}
	_, err = awssvc.SendMessage(&sqs.SendMessageInput{
		MessageBody:       aws.String(messageBody),
		MessageAttributes: messageAttributes,
		QueueUrl:          queueURL,
	})
	return err
}

// misc -----------------------------------------------------------------------

func (ta *TickerAttribute) getByUniqueKey(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx(`SELECT * FROM ticker_attribute WHERE ticker_id=? AND attribute_name=?`, ta.TickerId, ta.AttributeName).StructScan(ta)
	return err
}

type ByTickerPriceDate TickerDailies

func (a ByTickerPriceDate) Len() int { return len(a.Days) }
func (a ByTickerPriceDate) Less(i, j int) bool {
	return a.Days[i].PriceDate < a.Days[j].PriceDate
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
	return td.PriceTime == "09:30:00" || td.PriceTime >= "16:00:00"
}

func (td *TickerDaily) checkByDate(deps *Dependencies) uint64 {
	db := deps.db

	var tickerDailyId uint64
	db.QueryRowx(`SELECT ticker_daily_id FROM ticker_daily WHERE ticker_id=? AND price_date=?`, td.TickerId, td.PriceDate).Scan(&tickerDailyId)
	return tickerDailyId
}

func (td *TickerDaily) create(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

	if td.Volume == 0 {
		// Refusing to add ticker daily with 0 volume
		return nil
	}

	var insert = "INSERT INTO ticker_daily SET ticker_id=?, price_date=?, price_time=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=?"
	_, err := db.Exec(insert, td.TickerId, td.PriceDate, td.PriceTime, td.OpenPrice, td.HighPrice, td.LowPrice, td.ClosePrice, td.Volume)
	if err != nil {
		sublog.Fatal().Err(err).Msg("failed on INSERT")
	}
	return err
}

func (td *TickerDaily) createOrUpdate(deps *Dependencies) error {
	db := deps.db

	if td.Volume == 0 {
		// Refusing to add ticker daily with 0 volume
		return nil
	}

	td.TickerDailyId = td.checkByDate(deps)
	if td.TickerDailyId == 0 {
		return td.create(deps)
	}

	var update = "UPDATE ticker_daily SET price_time=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=? WHERE ticker_id=? AND price_date=?"
	_, err := db.Exec(update, td.PriceTime, td.OpenPrice, td.HighPrice, td.LowPrice, td.ClosePrice, td.Volume, td.TickerId, td.PriceDate)
	if err != nil {
		log.Warn().Err(err).Msg("failed on UPDATE")
	}
	return err
}

func getLastTickerDailyMove(deps *Dependencies, ticker_id uint64) (string, error) {
	db := deps.db

	var lastTickerDailyMove string
	row := db.QueryRowx(
		`SELECT IF(ticker_daily.close_price > prev_daily.close_price,"up",
		        IF(ticker_daily.close_price < prev_daily.close_price,"down",
				IF(ticker_daily.close_price = prev_daily.close_price,"unchanged", "unknown"))) AS lastTickerDailyMove
		 FROM ticker_daily
		 LEFT JOIN (
		   SELECT ticker_id, close_price FROM ticker_daily AS prev_ticker_daily
			 WHERE ticker_id=? ORDER by price_date DESC LIMIT 1,1
		 ) AS prev_daily ON (ticker_daily.ticker_id = prev_daily.ticker_id)
		 WHERE ticker_daily.ticker_id=?
		 ORDER BY price_date DESC
		 LIMIT 2`,
		ticker_id, ticker_id)
	err := row.Scan(&lastTickerDailyMove)
	return lastTickerDailyMove, err
}

// load last ticker price
func getLastTickerDaily(deps *Dependencies, ticker_id uint64) ([]TickerDaily, error) {
	db := deps.db
	sublog := deps.logger

	lastTickerDaily := []TickerDaily{}
	rows, err := db.Queryx(`SELECT * FROM ticker_daily WHERE ticker_daily.ticker_id=? ORDER BY price_date DESC LIMIT 2`, ticker_id)
	if err != nil {
		sublog.Fatal().Err(err).Uint64("ticker_id", ticker_id).Msg("failed to select last 2 ticker_daily records")
		return []TickerDaily{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var tickerDaily TickerDaily
		err := rows.StructScan(&tickerDaily)
		if err != nil {
			sublog.Fatal().Err(err).Uint64("ticker_id", ticker_id).Msg("failed to scan ticker_daily into struct")
			continue
		}
		tickerDaily.PriceDatetime, _ = time.Parse(sqlDatetimeParseType, tickerDaily.PriceDate[:11]+tickerDaily.PriceTime+"Z")
		lastTickerDaily = append(lastTickerDaily, tickerDaily)
	}
	if len(lastTickerDaily) != 2 {
		sublog.Error().Err(err).Uint64("ticker_id", ticker_id).Msg("failed to load 2 ticker_daily records into array")
	}

	return lastTickerDaily, nil
}

func (td *TickerDescription) getByUniqueKey(deps *Dependencies) error {
	db := deps.db

	err := db.QueryRowx(`SELECT * FROM ticker_description WHERE ticker_id=?`, td.TickerId).StructScan(td)
	return err
}

func (td *TickerDescription) createOrUpdate(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

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

// misc -----------------------------------------------------------------------

func getTickerDescriptionByTickerId(deps *Dependencies, ticker_id uint64) (*TickerDescription, error) {
	db := deps.db

	var tickerDescription TickerDescription
	err := db.QueryRowx(`SELECT * FROM ticker_description WHERE ticker_id=?`, ticker_id).StructScan(&tickerDescription)
	return &tickerDescription, err
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

func (tud *TickerUpDown) createIfNew(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

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

func (ts *TickerSplit) createIfNew(deps *Dependencies) error {
	db := deps.db
	sublog := deps.logger

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

func (t *Ticker) GetFinancials(deps *Dependencies, period, chartType string, isPercentage int) ([]string, []map[string]float64, error) {
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

func (t Ticker) queueSaveFavIcon(deps *Dependencies) error {
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

func (t *Ticker) getFavIconCDATA(deps *Dependencies) string {
	awssess := deps.awssess
	sublog := deps.logger

	if t.FavIconS3Key == "none" {
		return ""
	}
	if t.FavIconS3Key == "" {
		t.queueSaveFavIcon(deps)
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
