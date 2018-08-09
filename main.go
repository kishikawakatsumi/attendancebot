package main

import (
	"fmt"
	"github.com/nlopes/slack"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"net/http"
	"os"
)

var (
	logger       *zap.Logger
	sugar        *zap.SugaredLogger
	clientID     string
	clientSecret string
)

func main() {
	os.Exit(_main(os.Args[1:]))
}

func _main(_ []string) int {
	logger, _ = zap.NewDevelopment()
	defer logger.Sync()
	sugar = logger.Sugar()

	app := FlagSet()
	app.Action = func(c *cli.Context) error {
		if c.String("config") == "" {
			return fmt.Errorf("required -c option")
		}

		config, err := LoadConfig(c.String("config"), c.String("region"))
		if err != nil {
			return fmt.Errorf("failed to load toml file: %s", err)
		}

		clientID = config.OAuthClientID
		clientSecret = config.OAuthClientSecret

		sugar.Infof("Start slack event listening")
		client := slack.New(config.BotToken)
		slackListener := &SlackListener{
			client: client,
			botID:  config.BotID,
		}
		go slackListener.ListenAndResponse()
		go slackListener.sendReminderMessage()

		http.Handle("/interaction", interactionHandler{
			slackClient:       client,
			verificationToken: config.VerificationToken,
		})

		sugar.Infof("Server listening on :%s", c.String("port"))
		if err := http.ListenAndServe(":"+c.String("port"), nil); err != nil {
			return fmt.Errorf("%s", err)
		}
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		sugar.Fatal(err)
		return 1
	}
	return 0
}
