package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/rs/zerolog/log"

	"github.com/weirdtangent/myaws"
)

func sendNotification(awssess *session.Session, topicName string, action string, notificationText string) (bool, error) {
	awssvc := sns.New(awssess)

	awsregion, awsaccount, err := myaws.AWSAccount(awssess)
	if err != nil {
		return false, err
	}

	arn := "arn:aws:sns:" + *awsregion + ":" + *awsaccount + ":stockwatch-" + topicName

	MessageAttributes := make(map[string]*sns.MessageAttributeValue)
	MessageAttributes["action"] = &sns.MessageAttributeValue{
		StringValue: aws.String(action),
		DataType:    aws.String("String"),
	}

	result, err := awssvc.Publish(&sns.PublishInput{
		Message:           aws.String(notificationText),
		MessageAttributes: MessageAttributes,
		TopicArn:          aws.String(arn),
	})
	if err != nil {
		log.Error().Err(err).
			Str("topic_name", topicName).
			Str("arn", arn).
			Str("message", notificationText).
			Msg("Sending SNS notification")
		return false, err
	}

	log.Info().
		Str("aws_message_id", *result.MessageId).
		Str("topic_name", topicName).
		Str("arn", arn).
		Str("message", notificationText).
		Msg("Sent SNS notification to topic")

	return true, nil
}
