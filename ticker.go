package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/weirdtangent/mytime"
)

type Ticker struct {
	TickerId       int64  `db:"ticker_id"`
	TickerSymbol   string `db:"ticker_symbol"`
	ExchangeId     int64  `db:"exchange_id"`
	TickerName     string `db:"ticker_name"`
	CompanyName    string `db:"company_name"`
	Address        string `db:"address"`
	City           string `db:"city"`
	State          string `db:"state"`
	Zip            string `db:"zip"`
	Country        string `db:"country"`
	Website        string `db:"website"`
	Phone          string `db:"phone"`
	Sector         string `db:"sector"`
	Industry       string `db:"industry"`
	FetchDatetime  string `db:"fetch_datetime"`
	CreateDatetime string `db:"create_datetime"`
	UpdateDatetime string `db:"update_datetime"`
}

type TickersEODTask struct {
	TaskAction string `json:"action"`
	TickerId   int64  `json:"ticker_id"`
	DaysBack   int    `json:"days_back"`
	Offset     int    `json:"offset"`
}

func (t *Ticker) getBySymbol(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)

	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=?", t.TickerSymbol).StructScan(t)
	return err
}

func (t *Ticker) checkBySymbol(ctx context.Context) int64 {
	db := ctx.Value("db").(*sqlx.DB)

	var tickerId int64
	db.QueryRowx("SELECT ticker_id FROM ticker WHERE ticker_symbol=?", t.TickerSymbol).Scan(&tickerId)
	return tickerId
}

func (t *Ticker) create(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)
	logger := log.Ctx(ctx)

	if t.TickerSymbol == "" {
		logger.Warn().Msg("Refusing to add ticker with blank symbol")
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
			Msg("Failed on INSERT")
		return err
	}
	t.TickerId, err = res.LastInsertId()
	if err != nil {
		logger.Fatal().Err(err).
			Str("table_name", "ticker").
			Str("symbol", t.TickerSymbol).
			Msg("Failed on LAST_INSERTID")
		return err
	}
	return nil
}

func (t *Ticker) getOrCreate(ctx context.Context) error {
	err := t.getBySymbol(ctx)
	if err != nil && t.TickerId == 0 {
		return t.create(ctx)
	}
	return err
}

func (t *Ticker) createOrUpdate(ctx context.Context) error {
	db := ctx.Value("db").(*sqlx.DB)
	logger := log.Ctx(ctx)

	if t.TickerSymbol == "" {
		logger.Warn().Msg("Refusing to add ticker with blank symbol")
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
			Msg("Failed on UPDATE")
	}
	return t.getBySymbol(ctx)
}

func (t *Ticker) createOrUpdateAttribute(ctx context.Context, attributeName string, attributeValue string) error {
	db := ctx.Value("db").(*sqlx.DB)

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
	if currentWeekday != time.Saturday && currentWeekday != time.Sunday && currentTimeStr >= "1600" && todayEOD == false {
		log.Info().Msgf("needEOD: Today is %s, it is %s, and we don't have that EOD, so YES", currentWeekday, currentTimeStr)
		return true
	}

	priorWorkDate := mytime.PriorWorkDate(currentDate)
	priorWorkDateStr := priorWorkDate.Format("2006-01-02")

	priorEOD := t.haveEODForDate(ctx, priorWorkDateStr)

	if priorEOD == false {
		log.Info().Msgf("needEOD: PriorWorkDay is %s, and we don't have that EOD, so YES", priorWorkDateStr)
		return true
	}

	// if we have one for the prior work day, we should have them going back a year
	return false
}

// we need to get two days of the most recent EOD prices for this ticker
// on a weekend, or a weekday before open, we need the last work day, and the day before
// on a weekday during market hours, or after close, we need the live quote and the prior work day
func (t *Ticker) getLastAndPriorClose(ctx context.Context) (*TickerDaily, *TickerDaily) {
	db := ctx.Value("db").(*sqlx.DB)

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
	db := ctx.Value("db").(*sqlx.DB)

	var count int
	err := db.QueryRowx("SELECT COUNT(*) FROM ticker_daily WHERE ticker_id=? AND price_date=?", t.TickerId, dateStr).Scan(&count)
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
			Msg("Failed to create task JSON for EOD update for ticker")
		return false
	}

	_, err = sendNotification(awssess, "tickers", taskAction, string(taskJSON))
	if err == nil {
		log.Info().
			Int64("ticker_id", t.TickerId).
			Msg("Sent task notification for EOD update for ticker")
	}

	// submit task for 1000 more EODs
	taskVars = TickersEODTask{taskAction, t.TickerId, 1000, 1000}
	taskJSON, err = json.Marshal(taskVars)
	log.Info().Msg(string(taskJSON))
	if err != nil {
		log.Error().Err(err).
			Int64("ticker_id", t.TickerId).
			Msg("Failed to create task JSON for EOD update for ticker")
		return true
	}

	_, err = sendNotification(awssess, "tickers", taskAction, string(taskJSON))
	if err == nil {
		log.Info().
			Int64("ticker_id", t.TickerId).
			Msg("Sent task notification for EOD update for ticker")
	}
	return true
}

func (t Ticker) getTickerEODs(ctx context.Context, days int) ([]TickerDaily, error) {
	db := ctx.Value("db").(*sqlx.DB)
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
				Msg("Error reading result rows")
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
	db := ctx.Value("db").(*sqlx.DB)

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
				Msg("Error reading result rows")
		} else {
			upDowns = append(upDowns, tickerUpDown)
		}
	}
	if err := rows.Err(); err != nil {
		return upDowns, err
	}

	return upDowns, nil
}

func (t *Ticker) getLastDaily(ctx context.Context) (*TickerDaily, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var tickerdaily TickerDaily
	err := db.QueryRowx(`SELECT * FROM ticker_daily WHERE ticker_id=? ORDER BY price_date DESC LIMIT 1`, t.TickerId).StructScan(&tickerdaily)
	return &tickerdaily, err
}

func (t Ticker) getAttributes(ctx context.Context) ([]TickerAttribute, error) {
	db := ctx.Value("db").(*sqlx.DB)
	logger := log.Ctx(ctx)

	var tickerAttribute TickerAttribute
	tickerAttributes := make([]TickerAttribute, 0)

	rows, err := db.Queryx(
		`SELECT * FROM ticker_attribute WHERE ticker_id=?`,
		t.TickerId)
	if err != nil {
		return tickerAttributes, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.StructScan(&tickerAttribute)
		if err != nil {
			logger.Warn().Err(err).
				Str("table_name", "ticker_attribute").
				Msg("Error reading result rows")
		} else {
			underscore_rx := regexp.MustCompile(`_`)
			tickerAttribute.AttributeName = string(underscore_rx.ReplaceAll([]byte(tickerAttribute.AttributeName), []byte(" ")))
			tickerAttribute.AttributeName = strings.Title(strings.ToLower(tickerAttribute.AttributeName))
			tickerAttributes = append(tickerAttributes, tickerAttribute)
		}
	}
	if err := rows.Err(); err != nil {
		return tickerAttributes, err
	}

	return tickerAttributes, nil
}

// misc -----------------------------------------------------------------------

func getTickerBySymbol(ctx context.Context, symbol string) (*Ticker, error) {
	db := ctx.Value("db").(*sqlx.DB)

	var ticker Ticker
	err := db.QueryRowx("SELECT * FROM ticker WHERE ticker_symbol=?", symbol).StructScan(&ticker)
	return &ticker, err
}
