package main

const (
	httpPort = 3001

	awsRegion            = "us-east-1"
	awsPrivateBucketName = "stockwatch-private"

	skipRedisChecks     = false // always skip the redis cache info
	skipLocalTickerInfo = false // always fetch ticker info from yhfinance

	sqlDateParseType      = "2006-01-02"
	sqlDatetimeParseType  = "2006-01-02T15:04:05Z"
	sqlDatetimeSearchType = "2006-01-02 15:04:05"
	fullDatetime          = "2006-01-02 15:04:05 MST"

	zoneDir = "/usr/share/zoneinfo/"

	minTickerReloadDelayOpen   = 1       //  1 minute
	minTickerReloadDelayClosed = 60 * 1  //  1 hour
	minTickerNewsDelay         = 60 * 1  //  1 hour
	minTickerFinancialsDelay   = 60 * 24 // 24 hours

	volumeUnits    = 1_000_000 // factor to reduce volume counts by when graphing
	maxRecentCount = 6         // limit watcher_recents
	debugging      = true      // output DEBUG level logs

)

func main() {
	deps := &Dependencies{}

	setupLogging(deps)
	setupAWS(deps)
	setupSecrets(deps)
	setupSessionStore(deps)
	setupOAuth(deps)
	setupTemplates(deps)

	startServer(deps)
}
