package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sns"

	"github.com/weirdtangent/myaws"
)

func sendNotification(deps *Dependencies, topicName string, action string, notificationText string) (bool, error) {
	awssess := deps.awssess
	sublog := deps.logger

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
		sublog.Error().Err(err).
			Str("topic_name", topicName).
			Str("arn", arn).
			Str("message", notificationText).
			Msg("Sending SNS notification")
		return false, err
	}

	sublog.Info().
		Str("aws_message_id", *result.MessageId).
		Str("topic_name", topicName).
		Str("arn", arn).
		Str("message", notificationText).
		Msg("Sent SNS notification to topic")

	return true, nil
}
