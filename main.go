package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	log "github.com/inconshreveable/log15"
	"github.com/quipo/statsd"

	"github.com/Altoros/tweets-fetcher/fetcher"
	"github.com/Altoros/tweets-fetcher/server"
)

var (
	defaultPort          = "8080"
	defaultLogLevel      = log.LvlInfo
	requiredEnvVariables = []string{
		"TWITTER_CONSUMER_KEY",
		"TWITTER_CONSUMER_SECRET",
		"TWITTER_CONSUMER_ACCESS_TOKEN",
		"TWITTER_CONSUMER_ACCESS_SECRET",
	}
)

func main() {
	logger := log.New("module", "main")
	logger.SetHandler(log.LvlFilterHandler(getloggerLvl(), log.StreamHandler(os.Stdout, log.JsonFormat())))

	var err error

	err = checkReqiredEnvVariables()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	statsdClient := statsdClient()
	err = statsdClient.CreateSocket()
	if err != nil {
		logger.Error("Failed to create statsd socket", "err", err)
		os.Exit(1)
	}

	twitterClient := twitterClient(
		os.Getenv("TWITTER_CONSUMER_KEY"),
		os.Getenv("TWITTER_CONSUMER_SECRET"),
		os.Getenv("TWITTER_CONSUMER_ACCESS_TOKEN"),
		os.Getenv("TWITTER_CONSUMER_ACCESS_SECRET"),
	)
	fetcher := fetcher.New(logger, twitterClient, statsdClient)

	server := server.New(logger, fetcher)
	errChan := make(chan error)
	go server.Start(errChan, getPort())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM)

	select {
	case err := <-errChan:
		logger.Error("Start server", "err", err)
	case sig := <-signalChan:
		logger.Info(fmt.Sprintf("Received signal %s. shutting down", sig))
		server.Stop()
	}
}

func getPort() string {
	if os.Getenv("PORT") == "" {
		return defaultPort
	}
	return os.Getenv("PORT")
}

func getloggerLvl() log.Lvl {
	levelStr := os.Getenv("LOG_LEVEL")
	lvl, err := log.LvlFromString(levelStr)
	if err != nil {
		return defaultLogLevel
	}
	return lvl
}

func checkReqiredEnvVariables() error {
	for _, variable := range requiredEnvVariables {
		if os.Getenv(variable) == "" {
			return fmt.Errorf("Env variable %s required, but is not set", variable)
		}
	}

	return nil
}

func statsdClient() statsd.Statsd {
	return &statsd.NoopClient{}
}

func twitterClient(consumerKey, consumerSecret, accessToken, accessSecret string) *twitter.Client {
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	httpClient := config.Client(oauth1.NoContext, token)
	return twitter.NewClient(httpClient)
}
