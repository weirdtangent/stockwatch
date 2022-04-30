package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/rs/zerolog/log"
)

func pingHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}

func JSONReportHandler(deps *Dependencies) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		awssess := deps.awssess

		s3svc := s3.New(awssess)

		EasternTZ, _ := time.LoadLocation("America/New_York")
		currentDateTime := time.Now().In(EasternTZ)
		currentMonth := currentDateTime.Format("2006-01")

		b, _ := ioutil.ReadAll(r.Body)
		cspReport := string(b)

		sha1Hash := sha1.New()
		io.WriteString(sha1Hash, cspReport)
		logKey := fmt.Sprintf("csp-violations/%s/%x", currentMonth, string(sha1Hash.Sum(nil)))

		inputPutObj := &s3.PutObjectInput{
			Body:   aws.ReadSeekCloser(strings.NewReader(cspReport)),
			Bucket: aws.String("stockwatch-graystorm"),
			Key:    aws.String(logKey),
		}

		_, err := s3svc.PutObject(inputPutObj)
		if err != nil {
			log.Warn().Err(err).Str("bucket", "stockwatch-graystorm").Str("key", logKey).Msg("Failed to upload to S3 bucket")
		}
	})
}
