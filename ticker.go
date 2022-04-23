package main

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/mytime"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Ticker struct {
	TickerId        int64  `db:"ticker_id"`
	TickerSymbol    string `db:"ticker_symbol"`
	ExchangeId      int64  `db:"exchange_id"`
	TickerName      string `db:"ticker_name"`
	CompanyName     string `db:"company_name"`
	Address         string `db:"address"`
	City            string `db:"city"`
	State           string `db:"state"`
	Zip             string `db:"zip"`
	Country         string `db:"country"`
	Website         string `db:"website"`
	Phone           string `db:"phone"`
	Sector          string `db:"sector"`
	Industry        string `db:"industry"`
	FetchDatetime   string `db:"fetch_datetime"`
	MSPerformanceId string `db:"ms_performance_id"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

type TickersEODTask struct {
	TaskAction string `json:"action"`
	TickerId   int64  `json:"ticker_id"`
	DaysBack   int    `json:"days_back"`
	Offset     int    `json:"offset"`
}

type TickerAttribute struct {
	TickerAttributeId int64  `db:"attribute_id"`
	TickerId          int64  `db:"ticker_id"`
	AttributeName     string `db:"attribute_name"`
	AttributeValue    string `db:"attribute_value"`
	CreateDatetime    string `db:"create_datetime"`
	UpdateDatetime    string `db:"update_datetime"`
}

type TickerDaily struct {
	TickerDailyId  int64   `db:"ticker_daily_id"`
	TickerId       int64   `db:"ticker_id"`
	PriceDate      string  `db:"price_date"`
	PriceTime      string  `db:"price_time"`
	OpenPrice      float64 `db:"open_price"`
	HighPrice      float64 `db:"high_price"`
	LowPrice       float64 `db:"low_price"`
	ClosePrice     float64 `db:"close_price"`
	Volume         float64 `db:"volume"`
	CreateDatetime string  `db:"create_datetime"`
	UpdateDatetime string  `db:"update_datetime"`
}

type TickerDailies struct {
	Days []TickerDaily
}

type TickerDescription struct {
	TickerDescriptionId int64  `db:"description_id"`
	TickerId            int64  `db:"ticker_id"`
	BusinessSummary     string `db:"business_summary"`
	CreateDatetime      string `db:"create_datetime"`
	UpdateDatetime      string `db:"update_datetime"`
}

type TickerIntraday struct {
	TickerIntradayId int64   `db:"intraday_id"`
	TickerId         int64   `db:"ticker_id"`
	PriceDate        string  `db:"price_date"`
	LastPrice        float64 `db:"last_price"`
	Volume           float64 `db:"volume"`
	CreateDatetime   string  `db:"create_datetime"`
	UpdateDatetime   string  `db:"update_datetime"`
}

type TickerIntradays struct {
	Moments []TickerIntraday
}

type TickerUpDown struct {
	TickerUpDownId  int64  `db:"updown_id"`
	TickerId        int64  `db:"ticker_id"`
	UpDownAction    string `db:"updown_action"`
	UpDownFromGrade string `db:"updown_fromgrade"`
	UpDownToGrade   string `db:"updown_tograde"`
	UpDownDate      string `db:"updown_date"`
	UpDownFirm      string `db:"updown_firm"`
	UpDownSince     string `db:"updown_since"`
	CreateDatetime  string `db:"create_datetime"`
	UpdateDatetime  string `db:"update_datetime"`
}

type TickerSplit struct {
	TickerSplitId  int64  `db:"ticker_split_id"`
	TickerId       int64  `db:"ticker_id"`
	SplitDate      string `db:"split_date"`
	SplitRatio     string `db:"split_ratio"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

func (t *Ticker) getBySymbol(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=?", t.TickerSymbol).StructScan(t)
	return err
}

func (t *Ticker) checkBySymbol(ctx context.Context) int64 {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var tickerId int64
	db.QueryRowx("SELECT ticker_id FROM ticker WHERE ticker_symbol=?", t.TickerSymbol).Scan(&tickerId)
	return tickerId
}

func (t *Ticker) create(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if t.TickerSymbol == "" {
		// logger.Warn().Msg("refusing to add ticker with blank symbol")
		return nil
	}

	var insert = "INSERT INTO ticker SET ticker_symbol=?, exchange_id=?, ticker_name=?, company_name=?, address=?, city=?, state=?, zip=?, country=?, website=?, phone=?, sector=?, industry=?"
	if t.FetchDatetime == "now()" {
		insert += ", fetch_datetime=now()"
	}
	res, err := db.Exec(insert, t.TickerSymbol, t.ExchangeId, t.TickerName, t.CompanyName, t.Address, t.City, t.State, t.Zip, t.Country, t.Website, t.Phone, t.Sector, t.Industry)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker").
			Str("ticker", t.TickerSymbol).
			Msg("failed on INSERT")
		return err
	}
	t.TickerId, err = res.LastInsertId()
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker").
			Str("symbol", t.TickerSymbol).
			Msg("failed on LAST_INSERTID")
		return err
	}
	return nil
}

func (t *Ticker) createOrUpdate(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if t.TickerSymbol == "" {
		// logger.Warn().Msg("refusing to add ticker with blank symbol")
		return nil
	}

	t.TickerId = t.checkBySymbol(ctx)
	if t.TickerId == 0 {
		return t.create(ctx)
	}

	var update = "UPDATE ticker SET exchange_id=?, ticker_name=?, company_name=?, address=?, city=?, state=?, zip=?, country=?, website=?, phone=?, sector=?, industry=?"
	if t.FetchDatetime == "now()" {
		update += ", fetch_datetime=now()"
	}
	update += " WHERE ticker_id=?"
	_, err := db.Exec(update, t.ExchangeId, t.TickerName, t.CompanyName, t.Address, t.City, t.State, t.Zip, t.Country, t.Website, t.Phone, t.Sector, t.Industry, t.TickerId)
	if err != nil {
		logger.Warn().Err(err).
			Str("table_name", "ticker").
			Str("ticker", t.TickerSymbol).
			Msg("failed on UPDATE")
	}
	return t.getBySymbol(ctx)
}

func (t *Ticker) createOrUpdateAttribute(ctx context.Context, attributeName string, attributeValue string) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var attribute = TickerAttribute{0, t.TickerId, attributeName, attributeValue, "", ""}
	err := attribute.getByUniqueKey(ctx)
	if err == nil {
		var update = "UPDATE ticker_attribute SET attribute_value=? WHERE ticker_id=? AND attribute_name=?"
		db.Exec(update, attributeValue, t.TickerId, attributeName)
		return nil
	}

	var insert = "INSERT INTO ticker_attribute SET ticker_id=?, attribute_name=?, attribute_value=?"
	db.Exec(insert, t.TickerId, attributeName, attributeValue)
	return nil
}

// if it is a workday after 4 and we don't have the EOD, or we don't have the prior workday EOD, get them
func (t *Ticker) needEODs(ctx context.Context) bool {
	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	currentTimeStr := currentDate.Format("1505")
	currentDateStr := currentDate.Format("2006-01-02")
	currentWeekday := currentDate.Weekday()

	todayEOD := t.haveEODForDate(ctx, currentDateStr)

	// if it's a workday and the market closed for the day and we don't have today's EOD, then YES
	if currentWeekday != time.Saturday && currentWeekday != time.Sunday && currentTimeStr >= "1600" && !todayEOD {
		return true
	}

	priorWorkDate := mytime.PriorWorkDate(currentDate)
	priorWorkDateStr := priorWorkDate.Format("2006-01-02")

	priorEOD := t.haveEODForDate(ctx, priorWorkDateStr)

	return !priorEOD
}

// we need to get two days of the most recent EOD prices for this ticker
// on a weekend, or a weekday before open, we need the last work day, and the day before
// on a weekday during market hours, or after close, we need the live quote and the prior work day
func (t *Ticker) getLastAndPriorClose(ctx context.Context) (*TickerDaily, *TickerDaily) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	EasternTZ, _ := time.LoadLocation("America/New_York")
	currentDate := time.Now().In(EasternTZ)
	currentWeekday := currentDate.Weekday()
	timeStr := currentDate.Format("1505")

	// Lets assume "today's" close
	lastCloseDate := currentDate
	lastCloseDateStr := currentDate.Format("2006-01-02")

	// but: up until 9:30am on weekdays or anytime on weekends, we want prior workday's close
	if currentWeekday == time.Saturday || currentWeekday == time.Sunday || timeStr < "0930" {
		lastCloseDate = mytime.PriorWorkDate(lastCloseDate)
		lastCloseDateStr = lastCloseDate.Format("2006-01-02")
	}

	var lastClose TickerDaily
	db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_id=? AND price_date<=? ORDER BY price_date DESC LIMIT 1`, t.TickerId, lastCloseDateStr).StructScan(&lastClose)

	// ok and now get the prior day to that
	lastCloseDate = mytime.PriorWorkDate(lastCloseDate)
	lastCloseDateStr = lastCloseDate.Format("2006-01-02")

	var priorClose TickerDaily
	db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_id=? AND price_date<=? ORDER BY price_date DESC LIMIT 1`, t.TickerId, lastCloseDateStr).StructScan(&priorClose)
	return &lastClose, &priorClose
}

func (t Ticker) haveEODForDate(ctx context.Context, dateStr string) bool {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

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

func (t Ticker) ScheduleEODUpdate(awssess *session.Session, db *sqlx.DB) bool {
	earliest, _, err := t.EarliestEOD(db)
	if err != nil {
		log.Error().Err(err).Msg("failed to find earliest EOD")
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
	log.Info().Msg(string(taskJSON))
	if err != nil {
		log.Error().Err(err).
			Int64("ticker_id", t.TickerId).
			Msg("failed to create task JSON for EOD update for ticker")
		return false
	}

	_, err = sendNotification(awssess, "tickers", taskAction, string(taskJSON))
	if err == nil {
		log.Info().
			Int64("ticker_id", t.TickerId).
			Msg("sent task notification for EOD update for ticker")
	}

	// submit task for 1000 more EODs
	taskVars = TickersEODTask{taskAction, t.TickerId, 1000, 1000}
	taskJSON, err = json.Marshal(taskVars)
	log.Info().Msg(string(taskJSON))
	if err != nil {
		log.Error().Err(err).
			Int64("ticker_id", t.TickerId).
			Msg("failed to create task JSON for EOD update for ticker")
		return true
	}

	_, err = sendNotification(awssess, "tickers", taskAction, string(taskJSON))
	if err == nil {
		log.Info().
			Int64("ticker_id", t.TickerId).
			Msg("sent task notification for EOD update for ticker")
	}
	return true
}

func (t Ticker) getTickerEODs(ctx context.Context, days int) ([]TickerDaily, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

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
			logger.Warn().Err(err).
				Str("table_name", "ticker_daily").
				Msg("error reading result rows")
		} else {
			dailies = append(dailies, ticker_daily)
		}
	}
	if err := rows.Err(); err != nil {
		return dailies, err
	}

	return dailies, nil
}

func (t Ticker) getUpDowns(ctx context.Context, daysAgo int) ([]TickerUpDown, error) {
	logger := log.Ctx(ctx)
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

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
			logger.Warn().Err(err).
				Str("table_name", "ticker_updown").
				Msg("error reading result rows")
		} else {
			upDowns = append(upDowns, tickerUpDown)
		}
	}
	if err := rows.Err(); err != nil {
		return upDowns, err
	}

	return upDowns, nil
}

func (t Ticker) getAttributes(ctx context.Context) ([]TickerAttribute, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

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
			logger.Warn().Err(err).
				Str("table_name", "ticker_attribute").
				Msg("error reading result rows")
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

func (t Ticker) getSplits(ctx context.Context) ([]TickerSplit, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

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
			logger.Warn().Err(err).
				Str("table_name", "ticker_attribute").
				Msg("error reading result rows")
		} else {
			tickerSplits = append(tickerSplits, tickerSplit)
		}

	}
	if err := rows.Err(); err != nil {
		return tickerSplits, err
	}

	return tickerSplits, nil
}

func (t Ticker) queueUpdateNews(ctx context.Context) error {
	awssess := ctx.Value(ContextKey("awssess")).(*session.Session)
	awssvc := sqs.New(awssess)
	queueName := "stockwatch-tickers-news"

	urlResult, err := awssvc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		return err
	}

	type TaskTickerNewsBody struct {
		TickerId     int64  `json:"ticker_id"`
		TickerSymbol string `json:"ticker_symbol"`
		ExchangeId   int64  `json:"exchange_id"`
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

func (t Ticker) queueUpdateFinancials(ctx context.Context) error {
	awssess := ctx.Value(ContextKey("awssess")).(*session.Session)
	awssvc := sqs.New(awssess)
	queueName := "stockwatch-tickers-financials"

	urlResult, err := awssvc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		return err
	}

	type TaskTickerNewsBody struct {
		TickerId     int64  `json:"ticker_id"`
		TickerSymbol string `json:"ticker_symbol"`
		ExchangeId   int64  `json:"exchange_id"`
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

func getTickerBySymbol(ctx context.Context, symbol string) (*Ticker, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var ticker Ticker
	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=?", symbol).StructScan(&ticker)
	return &ticker, err
}

func getTickerById(ctx context.Context, ticker_id int64) (*Ticker, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var ticker Ticker
	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_id=?", ticker_id).StructScan(&ticker)
	return &ticker, err
}

func (ta *TickerAttribute) getByUniqueKey(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

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

func (td *TickerDaily) checkByDate(ctx context.Context) int64 {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var tickerDailyId int64
	db.QueryRowx(`SELECT ticker_daily_id FROM ticker_daily WHERE ticker_id=? AND price_date=?`, td.TickerId, td.PriceDate).Scan(&tickerDailyId)
	return tickerDailyId
}

func (td *TickerDaily) create(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if td.Volume == 0 {
		// logger.Warn().Msg("Refusing to add ticker daily with 0 volume")
		return nil
	}

	var insert = "INSERT INTO ticker_daily SET ticker_id=?, price_date=?, price_time=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=?"
	_, err := db.Exec(insert, td.TickerId, td.PriceDate, td.PriceTime, td.OpenPrice, td.HighPrice, td.LowPrice, td.ClosePrice, td.Volume)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker_daily").
			Msg("Failed on INSERT")
	}
	return err
}

func (td *TickerDaily) createOrUpdate(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if td.Volume == 0 {
		// logger.Warn().Msg("Refusing to add ticker daily with 0 volume")
		return nil
	}

	td.TickerDailyId = td.checkByDate(ctx)
	if td.TickerDailyId == 0 {
		return td.create(ctx)
	}

	var update = "UPDATE ticker_daily SET price_time=?, open_price=?, high_price=?, low_price=?, close_price=?, volume=? WHERE ticker_id=? AND price_date=?"
	_, err := db.Exec(update, td.PriceTime, td.OpenPrice, td.HighPrice, td.LowPrice, td.ClosePrice, td.Volume, td.TickerId, td.PriceDate)
	if err != nil {
		logger.Warn().Err(err).
			Str("table_name", "ticker_daily").
			Msg("Failed on UPDATE")
	}
	return err
}

func getLastTickerDailyMove(ctx context.Context, ticker_id int64) (string, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var lastTickerDailyMove string
	row := db.QueryRowx(
		`SELECT IF(ticker_daily.close_price >= prev.close_price,"up",IF(ticker_daily.close_price < prev.close_price,"down","unknown")) AS lastTickerDailyMove
		 FROM ticker_daily
		 LEFT JOIN (
		   SELECT ticker_id, close_price FROM ticker_daily AS prev_ticker_daily
			 WHERE ticker_id=? ORDER by price_date DESC LIMIT 1,1
		 ) AS prev ON (ticker_daily.ticker_id = prev.ticker_id)
		 WHERE ticker_daily.ticker_id=? ORDER BY price_date DESC LIMIT 1`,
		ticker_id, ticker_id)
	err := row.Scan(&lastTickerDailyMove)
	return lastTickerDailyMove, err
}

func (td *TickerDescription) getByUniqueKey(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM ticker_description WHERE ticker_id=?`, td.TickerId).StructScan(td)
	return err
}

func (td *TickerDescription) createIfNew(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if td.BusinessSummary == "" {
		return nil
	}

	err := td.getByUniqueKey(ctx)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_description SET ticker_id=?, business_summary=?"
	_, err = db.Exec(insert, td.TickerId, td.BusinessSummary)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker_description").
			Msg("Failed on INSERT")
	}
	return err
}

// misc -----------------------------------------------------------------------

func getTickerDescriptionByTickerId(ctx context.Context, ticker_id int64) (*TickerDescription, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

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

func (tud *TickerUpDown) getByUniqueKey(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM ticker_updown WHERE ticker_id=? AND updown_date=? AND updown_firm=?`, tud.TickerId, tud.UpDownDate, tud.UpDownFirm).StructScan(tud)
	return err
}

func (tud *TickerUpDown) createIfNew(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if tud.UpDownToGrade == "" {
		return nil
	}

	// if already exists, just quietly return
	err := tud.getByUniqueKey(ctx)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_updown SET ticker_id=?, updown_action=?, updown_fromgrade=?, updown_tograde=?, updown_date=?, updown_firm=?"
	_, err = db.Exec(insert, tud.TickerId, tud.UpDownAction, tud.UpDownFromGrade, tud.UpDownToGrade, tud.UpDownDate, tud.UpDownFirm)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker_updown").
			Msg("Failed on INSERT")
	}
	return err
}

func (ts *TickerSplit) getByDate(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	err := db.QueryRowx(`SELECT * FROM ticker_split WHERE ticker_id=? AND split_date=?`, ts.TickerId, ts.SplitDate).StructScan(ts)
	return err
}

func (ts *TickerSplit) createIfNew(ctx context.Context) error {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)
	logger := log.Ctx(ctx)

	if ts.SplitRatio == "" {
		// logger.Warn().Msg("Refusing to add ticker split with blank ratio")
		return nil
	}

	err := ts.getByDate(ctx)
	if err == nil {
		return nil
	}

	var insert = "INSERT INTO ticker_split SET ticker_id=?, split_date=?, split_ratio=?"
	_, err = db.Exec(insert, ts.TickerId, ts.SplitDate, ts.SplitRatio)
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker_split").
			Msg("Failed on INSERT")
	}
	return err
}

func (t *Ticker) GetFinancials(ctx context.Context, chartType string, period string) ([]string, []map[string]float64, error) {
	db := ctx.Value(ContextKey("db")).(*sqlx.DB)

	var periodStrs = []string{}
	var barValues = []map[string]float64{}

	rows, err := db.Queryx(`SELECT chart_date_string, group_concat(chart_name) AS chart_names,
	                          group_concat(chart_value) AS chart_values
						    FROM financials WHERE ticker_id=? and form_term_name=? and chart_type=?
							GROUP BY 1`,
		t.TickerId, period, chartType)
	if err != nil {
		return periodStrs, barValues, err
	}
	defer rows.Close()

	var financials struct {
		ChartDateString string `db:"chart_date_string"`
		ChartNames      string `db:"chart_names"`
		ChartValues     string `db:"chart_values"`
	}
	for rows.Next() {
		err = rows.StructScan(&financials)
		if err != nil {
			log.Warn().Err(err).
				Str("table_name", "financials").
				Msg("error reading result rows")
		} else {
			var barTime string
			if period == "quarterly" {
				quarterTime, _ := time.Parse("1/2006", financials.ChartDateString)
				barTime = quarterTime.Format("2006-01")
			} else {
				barTime = financials.ChartDateString
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

// type ByQuarter BarFinancials

// func (a ByQuarter) Len() int { return len(a.Quarterly) }
// func (a ByQuarter) Less(i, j int) bool {
// 	return a.Days[i].PriceDate < a.Days[j].PriceDate
// }
// func (a ByQuarter) Swap(i, j int) { a.Days[i], a.Days[j] = a.Days[j], a.Days[i] }

// func (td ByQuarter) Sort() *TickerDailies {
// 	sort.Sort(ByTickerPriceDate(td))
// 	return &td
// }
