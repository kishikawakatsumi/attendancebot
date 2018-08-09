package main

import (
	"github.com/kelseyhightower/envconfig"
	toml "github.com/sioncojp/tomlssm"
)

type Config struct {
	BotToken          string
	VerificationToken string
	BotID             string
	OAuthClientID     string
	OAuthClientSecret string
}

type envConfig struct {
	BotToken          string `envconfig:"BOT_TOKEN"`
	VerificationToken string `envconfig:"VERIFICATION_TOKEN"`
	BotID             string `envconfig:"BOT_ID"`
	OAuthClientID     string `envconfig:"OAUTH_CLIENT_ID"`
	OAuthClientSecret string `envconfig:"OAUTH_CLIENT_SECRET"`
}

type tomlConfig struct {
	BotToken          string `toml:"bot_token"`
	VerificationToken string `toml:"verification_token"`
	BotID             string `toml:"bot_id"`
	OAuthClientID     string `toml:"oauth_client_id"`
	OAuthClientSecret string `toml:"oauth_client_secret"`
}

func LoadConfig(path, region string) (*Config, error) {
	var config Config

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		sugar.Errorf("Failed to process env var: %s", err)
		return nil, err
	}

	tc, err := loadToml(path, region)
	if err != nil {
		sugar.Errorf("Failed to load 'config.toml': %s", err)
		return nil, err
	}

	config.BotToken = tc.BotToken
	if env.BotToken != "" {
		config.BotToken = env.BotToken
	}
	config.VerificationToken = tc.VerificationToken
	if env.VerificationToken != "" {
		config.VerificationToken = env.VerificationToken
	}
	config.BotID = tc.BotID
	if env.BotID != "" {
		config.BotID = env.BotID
	}
	config.OAuthClientID = tc.OAuthClientID
	if env.OAuthClientID != "" {
		config.OAuthClientID = env.OAuthClientID
	}
	config.OAuthClientSecret = tc.OAuthClientSecret
	if env.OAuthClientSecret != "" {
		config.OAuthClientSecret = env.OAuthClientSecret
	}

	return &config, nil
}

func loadToml(path, region string) (*tomlConfig, error) {
	var config tomlConfig
	if _, err := toml.DecodeFile(path, &config, region); err != nil {
		return nil, err
	}
	return &config, nil
}
