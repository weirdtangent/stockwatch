package main

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type TaskTickerBody struct {
	TickerId     uint64 `json:"ticker_id"`
	EId          string
	TickerSymbol string `json:"ticker_symbol"`
	ExchangeId   uint64 `json:"exchange_id"`
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

	// get next message from queue, if any
	queueURL := urlResult.QueueUrl
	messageBytes, _ := json.Marshal(TaskTickerBody{TickerSymbol: t.TickerSymbol})
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

	// get next message from queue, if any
	queueURL := urlResult.QueueUrl
	messageBytes, _ := json.Marshal(TaskTickerBody{TickerSymbol: t.TickerSymbol})
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

	// get next message from queue, if any
	queueURL := urlResult.QueueUrl
	messageBytes, _ := json.Marshal(TaskTickerBody{TickerSymbol: t.TickerSymbol})
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
