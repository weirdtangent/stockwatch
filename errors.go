package main

import "errors"

var (
	errFailedLoadSession             = errors.New("Failed to load session from DDB")
	errFailedSaveSession             = errors.New("Failed to save session to DDB")
	errFailedToGetSessionFromContext = errors.New("Failed to get session from Context")
)
