package main

import (
	"strings"

	"github.com/spf13/viper"

	"github.com/prasenjit/go-virtual/internal/ai"
	"github.com/prasenjit/go-virtual/internal/config"
)

func loadAIConfig() ai.Config {
	return ai.Config{
		Provider: strings.TrimSpace(viper.GetString("ai.provider")),
		OpenAI: ai.ProviderConfig{
			APIKey:  firstNonEmpty(viper.GetString("ai.openai.apiKey"), viper.GetString("ai.openaiApiKey")),
			Model:   firstNonEmpty(viper.GetString("ai.openai.model"), viper.GetString("ai.openaiModel")),
			BaseURL: firstNonEmpty(viper.GetString("ai.openai.baseUrl"), viper.GetString("ai.openaiBaseUrl")),
		},
		Claude: ai.ClaudeProviderConfig{
			APIKey:     strings.TrimSpace(viper.GetString("ai.claude.apiKey")),
			Model:      strings.TrimSpace(viper.GetString("ai.claude.model")),
			BaseURL:    strings.TrimSpace(viper.GetString("ai.claude.baseUrl")),
			APIVersion: strings.TrimSpace(viper.GetString("ai.claude.apiVersion")),
		},
	}
}

func loadSessionConfig() config.SessionConfig {
	cfg := config.SessionConfig{
		StoreType:         strings.TrimSpace(viper.GetString("session.storeType")),
		HeaderName:        viper.GetString("session.headerName"),
		InactivityTimeout: viper.GetDuration("session.inactivityTimeout"),
		MaxSessions:       viper.GetInt("session.maxSessions"),
		Redis: config.RedisSessionConfig{
			Addr:      strings.TrimSpace(viper.GetString("session.redis.addr")),
			Username:  strings.TrimSpace(viper.GetString("session.redis.username")),
			Password:  strings.TrimSpace(viper.GetString("session.redis.password")),
			DB:        viper.GetInt("session.redis.db"),
			KeyPrefix: strings.TrimSpace(viper.GetString("session.redis.keyPrefix")),
		},
	}
	cfg.Normalize()
	return cfg
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
