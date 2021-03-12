package main

import (
	//"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/myaws"
)

func main() {
	// setup logging -------------------------------------------------------------
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	// alter the caller() return to only include the last directory
	zerolog.CallerMarshalFunc = func(file string, line int) string {
		parts := strings.Split(file, "/")
		if len(parts) > 1 {
			return strings.Join(parts[len(parts)-2:], "/") + ":" + strconv.Itoa(line)
		}
		return file + ":" + strconv.Itoa(line)
	}
	log.Logger = log.With().Caller().Logger()

	// grab config ---------------------------------------------------------------
	//awsConfig, err := myaws.AWSConfig("us-east-1")

	// connect to AWS
	awssess, err := myaws.AWSConnect("us-east-1", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to AWS")
	}

	// connect to Aurora
	db, err := myaws.DBConnect(awssess, "stockwatch_rds", "stockwatch")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to RDS")
	}

	// handle cmd line params
	//listenAddr := flag.String("addr", ":3001", "HTTP listen address and port")
	//flag.Parse()

	mainloop(awssess, db)
}

func mainloop(awssess *session.Session, db *sqlx.DB) {
	var sleepTime float64 = 5
	for {
		processed, err := gettask(awssess, db, "stockwatch-tickers-eod")
		if err != nil {
			log.Fatal().Err(err).Msg("Fatal error, aborting loop")
		}
		if processed == true && sleepTime > 10 {
			sleepTime = sleepTime / 2
		} else if processed == false && sleepTime < (30*60) {
			sleepTime = sleepTime * 1.10
		}
		log.Info().Float64("sleep_seconds", sleepTime).Msg("sleeping...")
		m, _ := time.ParseDuration(fmt.Sprintf("%.0fs", sleepTime))
		time.Sleep(m)
	}
}

func gettask(awssess *session.Session, db *sqlx.DB, queueName string) (bool, error) {
	awssvc := sqs.New(awssess)
	tasklog := log.With().Str("queue", queueName).Logger()

	urlResult, err := awssvc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		tasklog.Error().Err(err).Msg("Failed to get URL for queue")
		return false, err
	}

	// get next message from queue, if any
	queueURL := urlResult.QueueUrl
	msgResult, err := awssvc.ReceiveMessage(&sqs.ReceiveMessageInput{
		AttributeNames: []*string{
			aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
		},
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
		QueueUrl:            queueURL,
		MaxNumberOfMessages: aws.Int64(1),
		VisibilityTimeout:   aws.Int64(60),
	})
	if err != nil {
		tasklog.Error().Err(err).Msg("Failed to get next message in queue")
		return false, err
	}
	if len(msgResult.Messages) == 0 {
		return false, nil
	}

	message := msgResult.Messages[0]
	messageHandle := message.ReceiptHandle

	action := *message.MessageAttributes["action"].StringValue
	body := msgResult.Messages[0].Body
	tasklog = tasklog.With().Str("action", action).Logger()
	tasklog.Info().Msg("Handling message in queue")

	// go handle whatever type of queued task this is
	var success bool

	switch action {
	case "eod":
		success, err = perform_tickers_eod(body)
	case "intraday":
		success, err = perform_tickers_intraday(body)
	default:
		success = false
		err = fmt.Errorf("unknown action: %s", action)
		tasklog.Error().Msg("Failed to understand action for this task type")
	}

	if err != nil {
		tasklog.Error().Err(err).Msg("Failed to process queued task, retrying won't help, deleting unprocessable task")
	} else if success == false {
		tasklog.Error().Err(err).Msg("Failed to process queued task successfully, but retryable")
		return true, nil
	}

	// if handled successfully, or not but retrying can't possibly help, delete message from queue
	_, err = awssvc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      queueURL,
		ReceiptHandle: messageHandle,
	})
	if err != nil {
		tasklog.Error().Err(err).Msg("Failed to delete message after processing")
		return true, nil
	}

	return true, nil
}

type TickersEODTask struct {
	TaskAction string `json:"action"`
	TickerId   int64  `json:"ticker_id"`
	DaysBack   int32  `json:"days_back"`
}

func perform_tickers_eod(body *string) (bool, error) {
	tasklog := log.With().Str("queue", "tickers").Str("action", "eod").Logger()

	if body == nil || body == "" {
		return false, fmt.Errorf("Failed to get JSON body in task")
	}

	var EODTask TickersEODTask
	json.NewDecoder(body).Decode(&EODTask)

	if EODTask.TaskAction != "eod" {
		return false, fmt.Errorf("Failed to decode JSON body in task")
	}
}

func perform_tickers_intraday(body *string) (bool, error) {
	if body != nil {
		return false, fmt.Errorf("Just testing")
	}
	return false, fmt.Errorf("Just testing")
}

func perform_tickers_deadletter(body *string) (bool, error) {
	if body != nil {
		return false, fmt.Errorf("Just testing")
	}
	return false, fmt.Errorf("Just testing")
}
