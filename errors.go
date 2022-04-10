package main

import "errors"

var (
	// errFailedLoadSession             = errors.New("failed to load session from DDB")
	// errFailedSaveSession             = errors.New("failed to save session to DDB")
	errFailedToGetSessionFromContext = errors.New("failed to get session from Context")
)
