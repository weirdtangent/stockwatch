package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/rs/zerolog"
)

type Contents struct {
	Content string
}

type Commit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Author struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		} `json:"author"`
		Message string `json:"message"`
	} `json:"commit"`
	Author struct {
		Login string `json:"login"`
		URL   string `json:"html_url"`
	} `json:"author"`
	URL string `json:"html_url"`
}

// object methods -------------------------------------------------------------

// misc -----------------------------------------------------------------------

func getGithubCommits(deps *Dependencies, sublog zerolog.Logger) (*string, *[]Commit, error) {
	secrets := deps.secrets

	var commitsResponse []Commit
	var readmeResponse Contents
	var readme string

	github_oauth_key := secrets["github_oauth_key"]

	url := "https://api.github.com/repos/weirdtangent/stockwatch/contents/README.md"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "token "+github_oauth_key)
	req.Header.Add("Accept", "application/json;charset=utf-8")

	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusOK {
		err := "failed to receive 200 success code from HTTP request"
		sublog.Error().Str("url", url).Int("status_code", res.StatusCode).Msg(err)
		return &readme, &commitsResponse, fmt.Errorf(err)
	}

	// request got a 200 response, lets read the results
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		sublog.Error().Err(err).Int("status_code", res.StatusCode).Msg("failed to ready body of response")
		return &readme, &commitsResponse, err
	}

	json.NewDecoder(strings.NewReader(string(body))).Decode(&readmeResponse)
	readmeMD, _ := base64.StdEncoding.DecodeString(readmeResponse.Content)
	readme = string(markdown.ToHTML([]byte(readmeMD), nil, nil))

	convertURLs := regexp.MustCompile(`(<a href="[^"]+")>`)
	readme = convertURLs.ReplaceAllString(readme, `$1 target="_new">`)

	url = "https://api.github.com/repos/weirdtangent/stockwatch/commits"
	req, _ = http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "token "+github_oauth_key)
	req.Header.Add("Accept", "application/json;charset=utf-8")

	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusOK {
		err := "failed to receive 200 success code from HTTP request"
		sublog.Error().Str("url", url).Int("status_code", res.StatusCode).Msg(err)
		return &readme, &commitsResponse, fmt.Errorf(err)
	}

	// request got a 200 response, lets read the results
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		sublog.Error().Err(err).Int("status_code", res.StatusCode).Msg("failed to ready body of response")
		return &readme, &commitsResponse, err
	}

	json.NewDecoder(strings.NewReader(string(body))).Decode(&commitsResponse)

	return &readme, &commitsResponse, nil
}
