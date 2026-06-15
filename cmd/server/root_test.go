package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/spf13/viper"
)

func TestSetDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setDefaults()

	if viper.GetInt("server.port") != 8080 {
		t.Fatalf("expected default port 8080")
	}
	if viper.GetString("server.host") != "0.0.0.0" {
		t.Fatalf("expected default host 0.0.0.0")
	}
	if viper.GetString("storage.type") != "file" {
		t.Fatalf("expected default storage type file")
	}
	if viper.GetInt("tracing.maxTraces") != 1000 {
		t.Fatalf("expected default tracing maxTraces 1000")
	}
	if viper.GetString("ai.provider") != "openai" {
		t.Fatalf("expected default AI provider openai, got %q", viper.GetString("ai.provider"))
	}
	if viper.GetString("ai.openai.model") != "gpt-4o-mini" {
		t.Fatalf("expected default OpenAI model gpt-4o-mini, got %q", viper.GetString("ai.openai.model"))
	}
	if viper.GetString("ai.claude.model") == "" {
		t.Fatalf("expected default Claude model to be set")
	}
	if viper.GetString("logging.level") != "info" {
		t.Fatalf("expected default logging level info, got %q", viper.GetString("logging.level"))
	}
	if viper.GetString("logging.format") != "json" {
		t.Fatalf("expected default logging format json, got %q", viper.GetString("logging.format"))
	}
	if viper.GetString("session.storeType") != config.SessionStoreMemory {
		t.Fatalf("expected default session store type %q, got %q", config.SessionStoreMemory, viper.GetString("session.storeType"))
	}
	if viper.GetString("session.redis.addr") != config.DefaultRedisAddr {
		t.Fatalf("expected default session redis addr %q, got %q", config.DefaultRedisAddr, viper.GetString("session.redis.addr"))
	}
	if viper.GetString("session.redis.keyPrefix") != config.DefaultRedisKeyPrefix {
		t.Fatalf("expected default session redis key prefix %q, got %q", config.DefaultRedisKeyPrefix, viper.GetString("session.redis.keyPrefix"))
	}

	cwd, _ := os.Getwd()
	expectedPath := filepath.Join(cwd, "data")
	if viper.GetString("storage.path") != expectedPath {
		t.Fatalf("expected default storage path %q, got %q", expectedPath, viper.GetString("storage.path"))
	}
}

func TestInitConfig_WithConfigFile(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	cfgFile = ""

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	config := []byte("server:\n  port: 9999\n")
	if err := os.WriteFile(configPath, config, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfgFile = configPath
	initConfig()

	if viper.GetInt("server.port") != 9999 {
		t.Fatalf("expected port 9999 from config, got %d", viper.GetInt("server.port"))
	}
}

func TestRunInit(t *testing.T) {
	origForce := initForce
	origPath := initPath
	defer func() {
		initForce = origForce
		initPath = origPath
	}()

	tempDir := t.TempDir()
	initPath = tempDir
	initForce = false

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit error: %v", err)
	}

	configPath := filepath.Join(tempDir, "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config.yaml to exist: %v", err)
	}
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected generated config.yaml to be readable: %v", err)
	}
	generated := string(configData)
	for _, want := range []string{
		"# Tips:",
		`# - Durations use Go duration syntax such as "30m", "24h", or "15s".`,
		"headless: false",
		"branding:",
		"scripting:",
		"session:",
		`storeType: "memory"`,
		`# Redis configuration — used only when storeType is "redis":`,
		"redis:",
		`addr: "127.0.0.1:6379"`,
		`keyPrefix: "go-virtual:sessions"`,
		"proxy:",
		"ai:",
		`provider: "openai"`,
		`model: "gpt-4o-mini"`,
		`model: "claude-sonnet-4-6"`,
		`# Which provider powers AI features: "openai", "claude", or "copilot".`,
		`path: "./data"`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("expected generated config to contain %q, got:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "openaiApiKey:") || strings.Contains(generated, "openaiModel:") || strings.Contains(generated, "openaiBaseUrl:") {
		t.Fatalf("expected generated config to omit legacy OpenAI aliases, got:\n%s", generated)
	}

	t.Chdir(tempDir)
	loadedCfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("expected generated config to load: %v", err)
	}
	if loadedCfg.Server.Host != "0.0.0.0" {
		t.Fatalf("expected generated host default 0.0.0.0, got %q", loadedCfg.Server.Host)
	}
	if loadedCfg.Storage.Path != filepath.Join(tempDir, "data") {
		t.Fatalf("expected generated storage path %q, got %q", filepath.Join(tempDir, "data"), loadedCfg.Storage.Path)
	}
	if loadedCfg.AI.Provider != config.AIProviderOpenAI {
		t.Fatalf("expected generated AI provider %q, got %q", config.AIProviderOpenAI, loadedCfg.AI.Provider)
	}
	if loadedCfg.AI.OpenAI.Model != config.DefaultOpenAIModel {
		t.Fatalf("expected generated OpenAI model %q, got %q", config.DefaultOpenAIModel, loadedCfg.AI.OpenAI.Model)
	}
	if loadedCfg.AI.Claude.Model != config.DefaultClaudeModel {
		t.Fatalf("expected generated Claude model %q, got %q", config.DefaultClaudeModel, loadedCfg.AI.Claude.Model)
	}
	if loadedCfg.Session.StoreType != config.SessionStoreMemory {
		t.Fatalf("expected generated session store type %q, got %q", config.SessionStoreMemory, loadedCfg.Session.StoreType)
	}
	if loadedCfg.Session.Redis.Addr != config.DefaultRedisAddr {
		t.Fatalf("expected generated session redis addr %q, got %q", config.DefaultRedisAddr, loadedCfg.Session.Redis.Addr)
	}
	if loadedCfg.Session.Redis.KeyPrefix != config.DefaultRedisKeyPrefix {
		t.Fatalf("expected generated session redis key prefix %q, got %q", config.DefaultRedisKeyPrefix, loadedCfg.Session.Redis.KeyPrefix)
	}

	dirs := []string{
		filepath.Join(tempDir, "data"),
		filepath.Join(tempDir, "data", "specs"),
		filepath.Join(tempDir, "data", "responses"),
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("expected directory to exist: %s", dir)
		}
	}

	if err := runInit(nil, nil); err == nil {
		t.Fatalf("expected error when config.yaml exists without --force")
	}

	initForce = true
	if err := runInit(nil, nil); err != nil {
		t.Fatalf("expected runInit with --force to succeed: %v", err)
	}
}

func TestLoadAIConfigAndFirstNonEmpty(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("ai.provider", config.AIProviderClaude)
	viper.Set("ai.openaiApiKey", " legacy-key ")
	viper.Set("ai.openaiModel", " legacy-model ")
	viper.Set("ai.openaiBaseUrl", " https://legacy.example ")
	viper.Set("ai.claude.apiKey", " claude-key ")
	viper.Set("ai.claude.model", " claude-model ")
	viper.Set("ai.claude.baseUrl", " https://claude.example ")
	viper.Set("ai.claude.apiVersion", " 2023-06-01 ")

	cfg := loadAIConfig()
	if cfg.Provider != config.AIProviderClaude {
		t.Fatalf("expected provider %q, got %q", config.AIProviderClaude, cfg.Provider)
	}
	if cfg.OpenAI.APIKey != "legacy-key" || cfg.OpenAI.Model != "legacy-model" || cfg.OpenAI.BaseURL != "https://legacy.example" {
		t.Fatalf("expected legacy OpenAI aliases to load, got %+v", cfg.OpenAI)
	}
	if cfg.Claude.APIKey != "claude-key" || cfg.Claude.Model != "claude-model" || cfg.Claude.BaseURL != "https://claude.example" || cfg.Claude.APIVersion != "2023-06-01" {
		t.Fatalf("expected Claude config to be trimmed and loaded, got %+v", cfg.Claude)
	}

	if got := firstNonEmpty("", "  ", " value ", "other"); got != "value" {
		t.Fatalf("expected first non-empty trimmed value, got %q", got)
	}
	if got := firstNonEmpty("", " "); got != "" {
		t.Fatalf("expected empty fallback, got %q", got)
	}
}

func TestLoadSessionConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("session.storeType", config.SessionStoreRedis)
	viper.Set("session.headerName", " X-Scale-Session ")
	viper.Set("session.inactivityTimeout", "45m")
	viper.Set("session.maxSessions", 250)
	viper.Set("session.redis.addr", " redis.example:6380 ")
	viper.Set("session.redis.username", " app-user ")
	viper.Set("session.redis.password", " secret ")
	viper.Set("session.redis.db", 3)
	viper.Set("session.redis.keyPrefix", " app:sessions ")

	cfg := loadSessionConfig()
	if cfg.StoreType != config.SessionStoreRedis {
		t.Fatalf("expected session store type %q, got %q", config.SessionStoreRedis, cfg.StoreType)
	}
	if cfg.HeaderName != "X-Scale-Session" {
		t.Fatalf("expected trimmed header name, got %q", cfg.HeaderName)
	}
	if cfg.InactivityTimeout.Minutes() != 45 {
		t.Fatalf("expected inactivity timeout 45m, got %v", cfg.InactivityTimeout)
	}
	if cfg.MaxSessions != 250 {
		t.Fatalf("expected maxSessions 250, got %d", cfg.MaxSessions)
	}
	if cfg.Redis.Addr != "redis.example:6380" {
		t.Fatalf("expected redis addr to load, got %q", cfg.Redis.Addr)
	}
	if cfg.Redis.Username != "app-user" || cfg.Redis.Password != "secret" || cfg.Redis.DB != 3 || cfg.Redis.KeyPrefix != "app:sessions" {
		t.Fatalf("expected redis config to load, got %+v", cfg.Redis)
	}
}

func TestLoadStorageConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setDefaults()

	// Defaults.
	if viper.GetString("storage.mongo.database") != config.DefaultMongoDB {
		t.Fatalf("expected default mongo database %q, got %q", config.DefaultMongoDB, viper.GetString("storage.mongo.database"))
	}
	if viper.GetString("storage.mongo.collectionPrefix") != config.DefaultMongoCollectionPrefix {
		t.Fatalf("expected default collection prefix %q, got %q", config.DefaultMongoCollectionPrefix, viper.GetString("storage.mongo.collectionPrefix"))
	}
	if viper.GetInt("storage.mongo.connectTimeoutSeconds") != config.DefaultMongoConnectTimeoutSeconds {
		t.Fatalf("expected default connect timeout %d, got %d", config.DefaultMongoConnectTimeoutSeconds, viper.GetInt("storage.mongo.connectTimeoutSeconds"))
	}
	if viper.GetString("storage.mongo.uri") != "" {
		t.Fatalf("expected default mongo URI to be empty, got %q", viper.GetString("storage.mongo.uri"))
	}

	// Override via viper.
	viper.Set("storage.type", "mongo")
	viper.Set("storage.mongo.uri", "mongodb://localhost:27017")
	viper.Set("storage.mongo.database", "testdb")

	if viper.GetString("storage.type") != "mongo" {
		t.Fatalf("expected storage type mongo, got %q", viper.GetString("storage.type"))
	}
	if viper.GetString("storage.mongo.uri") != "mongodb://localhost:27017" {
		t.Fatalf("expected mongo URI to be set, got %q", viper.GetString("storage.mongo.uri"))
	}
	if viper.GetString("storage.mongo.database") != "testdb" {
		t.Fatalf("expected mongo database %q, got %q", "testdb", viper.GetString("storage.mongo.database"))
	}
}
