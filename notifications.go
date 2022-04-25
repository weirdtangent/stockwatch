package main

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/rs/zerolog"

	"github.com/weirdtangent/myaws"
)

func sendNotification(ctx context.Context, topicName string, action string, notificationText string) (bool, error) {
	awssess := ctx.Value(ContextKey("awssess")).(*session.Session)
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
		zerolog.Ctx(ctx).Error().Err(err).
			Str("topic_name", topicName).
			Str("arn", arn).
			Str("message", notificationText).
			Msg("Sending SNS notification")
		return false, err
	}

	zerolog.Ctx(ctx).Info().
		Str("aws_message_id", *result.MessageId).
		Str("topic_name", topicName).
		Str("arn", arn).
		Str("message", notificationText).
		Msg("Sent SNS notification to topic")

	return true, nil
}
