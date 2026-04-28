package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Server defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got %q", cfg.Server.Host)
	}

	// Storage defaults
	if cfg.Storage.Type != "file" {
		t.Errorf("Expected default storage type 'file', got %q", cfg.Storage.Type)
	}
	// Default path should be an absolute path ending with "data"
	if !filepath.IsAbs(cfg.Storage.Path) {
		t.Errorf("Expected default storage path to be absolute, got %q", cfg.Storage.Path)
	}
	if filepath.Base(cfg.Storage.Path) != "data" {
		t.Errorf("Expected default storage path to end with 'data', got %q", cfg.Storage.Path)
	}

	// Tracing defaults
	if cfg.Tracing.MaxTraces != 1000 {
		t.Errorf("Expected default max traces 1000, got %d", cfg.Tracing.MaxTraces)
	}
	if cfg.Tracing.Retention != 24*time.Hour {
		t.Errorf("Expected default retention 24h, got %v", cfg.Tracing.Retention)
	}

	// Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default log level 'info', got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Expected default log format 'json', got %q", cfg.Logging.Format)
	}
	if cfg.AI.Provider != AIProviderOpenAI {
		t.Errorf("Expected default AI provider %q, got %q", AIProviderOpenAI, cfg.AI.Provider)
	}
	if cfg.AI.OpenAI.Model != DefaultOpenAIModel {
		t.Errorf("Expected default OpenAI model %q, got %q", DefaultOpenAIModel, cfg.AI.OpenAI.Model)
	}
	if cfg.AI.Claude.Model != DefaultClaudeModel {
		t.Errorf("Expected default Claude model %q, got %q", DefaultClaudeModel, cfg.AI.Claude.Model)
	}
	if cfg.AI.Claude.APIVersion != DefaultClaudeAPIVersion {
		t.Errorf("Expected default Claude API version %q, got %q", DefaultClaudeAPIVersion, cfg.AI.Claude.APIVersion)
	}
	if cfg.Session.StoreType != SessionStoreMemory {
		t.Errorf("Expected default session store type %q, got %q", SessionStoreMemory, cfg.Session.StoreType)
	}
	if cfg.Session.Redis.Addr != DefaultRedisAddr {
		t.Errorf("Expected default session redis addr %q, got %q", DefaultRedisAddr, cfg.Session.Redis.Addr)
	}
	if cfg.Session.Redis.KeyPrefix != DefaultRedisKeyPrefix {
		t.Errorf("Expected default session redis key prefix %q, got %q", DefaultRedisKeyPrefix, cfg.Session.Redis.KeyPrefix)
	}
}

func TestLoad(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 9090
  host: localhost
storage:
  type: file
  path: /tmp/data
tracing:
  maxTraces: 500
  retention: 12h
logging:
  level: debug
  format: text
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify loaded values
	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got %q", cfg.Server.Host)
	}
	if cfg.Storage.Type != "file" {
		t.Errorf("Expected storage type 'file', got %q", cfg.Storage.Type)
	}
	if cfg.Storage.Path != "/tmp/data" {
		t.Errorf("Expected storage path '/tmp/data', got %q", cfg.Storage.Path)
	}
	if cfg.Tracing.MaxTraces != 500 {
		t.Errorf("Expected max traces 500, got %d", cfg.Tracing.MaxTraces)
	}
	if cfg.Tracing.Retention != 12*time.Hour {
		t.Errorf("Expected retention 12h, got %v", cfg.Tracing.Retention)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Expected log format 'text', got %q", cfg.Logging.Format)
	}
}

func TestLoad_AIProviderConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
ai:
  provider: claude
  claude:
    apiKey: test-claude-key
    model: claude-3-7-sonnet-latest
    baseUrl: https://api.anthropic.com/v1/messages
    apiVersion: 2023-06-01
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.AI.Provider != AIProviderClaude {
		t.Fatalf("Expected provider %q, got %q", AIProviderClaude, cfg.AI.Provider)
	}
	if cfg.AI.Claude.APIKey != "test-claude-key" {
		t.Errorf("Expected Claude API key to load")
	}
	if cfg.AI.Claude.Model != "claude-3-7-sonnet-latest" {
		t.Errorf("Expected Claude model override, got %q", cfg.AI.Claude.Model)
	}
	if cfg.AI.Claude.BaseURL != "https://api.anthropic.com/v1/messages" {
		t.Errorf("Expected Claude base URL override, got %q", cfg.AI.Claude.BaseURL)
	}
}

func TestLoad_SessionRedisConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
session:
  storeType: redis
  headerName: X-Scale-Session
  inactivityTimeout: 45m
  maxSessions: 500
  redis:
    addr: redis.example:6380
    username: app-user
    password: secret
    db: 2
    keyPrefix: app:sessions
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Session.StoreType != SessionStoreRedis {
		t.Fatalf("Expected session store type %q, got %q", SessionStoreRedis, cfg.Session.StoreType)
	}
	if cfg.Session.HeaderName != "X-Scale-Session" {
		t.Fatalf("Expected session header name to load, got %q", cfg.Session.HeaderName)
	}
	if cfg.Session.InactivityTimeout != 45*time.Minute {
		t.Fatalf("Expected inactivity timeout 45m, got %v", cfg.Session.InactivityTimeout)
	}
	if cfg.Session.MaxSessions != 500 {
		t.Fatalf("Expected maxSessions 500, got %d", cfg.Session.MaxSessions)
	}
	if cfg.Session.Redis.Addr != "redis.example:6380" || cfg.Session.Redis.Username != "app-user" || cfg.Session.Redis.Password != "secret" || cfg.Session.Redis.DB != 2 || cfg.Session.Redis.KeyPrefix != "app:sessions" {
		t.Fatalf("Expected Redis session config to load, got %+v", cfg.Session.Redis)
	}
}

func TestLoad_AILegacyOpenAIAliases(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
ai:
  openaiApiKey: legacy-key
  openaiModel: gpt-4.1-mini
  openaiBaseUrl: http://localhost:1234/v1
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.AI.Provider != AIProviderOpenAI {
		t.Fatalf("Expected provider %q, got %q", AIProviderOpenAI, cfg.AI.Provider)
	}
	if cfg.AI.OpenAI.APIKey != "legacy-key" {
		t.Errorf("Expected legacy OpenAI API key to map into nested config")
	}
	if cfg.AI.OpenAI.Model != "gpt-4.1-mini" {
		t.Errorf("Expected legacy OpenAI model to map into nested config, got %q", cfg.AI.OpenAI.Model)
	}
	if cfg.AI.OpenAI.BaseURL != "http://localhost:1234/v1" {
		t.Errorf("Expected legacy OpenAI base URL to map into nested config, got %q", cfg.AI.OpenAI.BaseURL)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	// Create temporary config file with partial config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Only override server port
	configContent := `
server:
  port: 3000
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify overridden value
	if cfg.Server.Port != 3000 {
		t.Errorf("Expected port 3000, got %d", cfg.Server.Port)
	}

	// Verify defaults are preserved
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got %q", cfg.Server.Host)
	}
	if cfg.Storage.Type != "file" {
		t.Errorf("Expected default storage type 'file', got %q", cfg.Storage.Type)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Invalid YAML
	configContent := `
server:
  port: [invalid yaml
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Empty file
	err := os.WriteFile(configPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Should have defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestServerConfig(t *testing.T) {
	cfg := ServerConfig{
		Port: 8080,
		Host: "0.0.0.0",
	}

	if cfg.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Expected host '0.0.0.0', got %q", cfg.Host)
	}
}

func TestStorageConfig(t *testing.T) {
	cfg := StorageConfig{
		Type: "file",
		Path: "/data",
	}

	if cfg.Type != "file" {
		t.Errorf("Expected type 'file', got %q", cfg.Type)
	}
	if cfg.Path != "/data" {
		t.Errorf("Expected path '/data', got %q", cfg.Path)
	}
}

func TestTracingConfig(t *testing.T) {
	cfg := TracingConfig{
		MaxTraces: 500,
		Retention: 6 * time.Hour,
	}

	if cfg.MaxTraces != 500 {
		t.Errorf("Expected max traces 500, got %d", cfg.MaxTraces)
	}
	if cfg.Retention != 6*time.Hour {
		t.Errorf("Expected retention 6h, got %v", cfg.Retention)
	}
}

func TestLoggingConfig(t *testing.T) {
	cfg := LoggingConfig{
		Level:  "warn",
		Format: "text",
	}

	if cfg.Level != "warn" {
		t.Errorf("Expected level 'warn', got %q", cfg.Level)
	}
	if cfg.Format != "text" {
		t.Errorf("Expected format 'text', got %q", cfg.Format)
	}
}

func TestLoggingConfigNormalize(t *testing.T) {
	cfg := LoggingConfig{
		Level:  " DEBUG ",
		Format: " TEXT ",
	}
	cfg.Normalize()

	if cfg.Level != LogLevelDebug {
		t.Fatalf("expected normalized debug level, got %q", cfg.Level)
	}
	if cfg.Format != LogFormatText {
		t.Fatalf("expected normalized text format, got %q", cfg.Format)
	}
}

func TestLoggingConfigNormalizeInvalidValues(t *testing.T) {
	cfg := LoggingConfig{
		Level:  "verbose",
		Format: "pretty",
	}
	cfg.Normalize()

	if cfg.Level != LogLevelInfo {
		t.Fatalf("expected invalid level to fall back to info, got %q", cfg.Level)
	}
	if cfg.Format != LogFormatJSON {
		t.Fatalf("expected invalid format to fall back to json, got %q", cfg.Format)
	}
}

func TestSessionConfigNormalize(t *testing.T) {
	cfg := SessionConfig{
		StoreType:         " REDIS ",
		HeaderName:        " X-Scale-Session ",
		InactivityTimeout: 45 * time.Minute,
		MaxSessions:       250,
		Redis: RedisSessionConfig{
			Addr:      " redis.example:6380 ",
			Username:  " user ",
			Password:  " pass ",
			DB:        3,
			KeyPrefix: " app:sessions ",
		},
	}
	cfg.Normalize()

	if cfg.StoreType != SessionStoreRedis {
		t.Fatalf("expected normalized redis store type, got %q", cfg.StoreType)
	}
	if cfg.HeaderName != "X-Scale-Session" {
		t.Fatalf("expected trimmed header name, got %q", cfg.HeaderName)
	}
	if cfg.Redis.Addr != "redis.example:6380" || cfg.Redis.Username != "user" || cfg.Redis.Password != "pass" || cfg.Redis.DB != 3 || cfg.Redis.KeyPrefix != "app:sessions" {
		t.Fatalf("expected trimmed redis config, got %+v", cfg.Redis)
	}
}

func TestSessionConfigNormalizeInvalidValues(t *testing.T) {
	cfg := SessionConfig{
		StoreType:         "clustered",
		HeaderName:        " ",
		InactivityTimeout: 0,
		MaxSessions:       0,
		Redis: RedisSessionConfig{
			Addr:      " ",
			DB:        -1,
			KeyPrefix: " ",
		},
	}
	cfg.Normalize()

	if cfg.StoreType != SessionStoreMemory {
		t.Fatalf("expected invalid store type to fall back to %q, got %q", SessionStoreMemory, cfg.StoreType)
	}
	if cfg.HeaderName != "X-Virtual-Session-Id" {
		t.Fatalf("expected default header name, got %q", cfg.HeaderName)
	}
	if cfg.InactivityTimeout != 30*time.Minute {
		t.Fatalf("expected default inactivity timeout 30m, got %v", cfg.InactivityTimeout)
	}
	if cfg.MaxSessions != 10000 {
		t.Fatalf("expected default maxSessions 10000, got %d", cfg.MaxSessions)
	}
	if cfg.Redis.Addr != DefaultRedisAddr || cfg.Redis.DB != 0 || cfg.Redis.KeyPrefix != DefaultRedisKeyPrefix {
		t.Fatalf("expected default redis config fallback, got %+v", cfg.Redis)
	}
}

func TestLoad_RelativeStoragePath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Use a relative path; Load() should convert it to absolute
	configContent := `
storage:
  type: file
  path: ./relative/data
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// The path should have been made absolute
	if !filepath.IsAbs(cfg.Storage.Path) {
		t.Errorf("Expected storage path to be absolute after Load(), got %q", cfg.Storage.Path)
	}
	if filepath.Base(cfg.Storage.Path) != "data" {
		t.Errorf("Expected path to end with 'data', got %q", cfg.Storage.Path)
	}
}

func TestLoad_TLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  tls:
    enabled: true
    certFile: /certs/server.crt
    keyFile: /certs/server.key
    autoGenerate: false
    storePath: /certs
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if !cfg.Server.TLS.Enabled {
		t.Error("Expected TLS to be enabled")
	}
	if cfg.Server.TLS.CertFile != "/certs/server.crt" {
		t.Errorf("Expected certFile '/certs/server.crt', got %q", cfg.Server.TLS.CertFile)
	}
	if cfg.Server.TLS.AutoGenerate {
		t.Error("Expected autoGenerate to be false")
	}
}

func TestDefaultTLSConfig(t *testing.T) {
	cfg := Default()
	if cfg.Server.TLS.Enabled {
		t.Error("Expected TLS to be disabled by default")
	}
	if !cfg.Server.TLS.AutoGenerate {
		t.Error("Expected autoGenerate to be true by default")
	}
}

func TestBrandingConfig(t *testing.T) {
	cfg := BrandingConfig{
		AppTitle:    "My API Tool",
		AppSubtitle: "Custom Subtitle",
	}

	if cfg.AppTitle != "My API Tool" {
		t.Errorf("Expected AppTitle 'My API Tool', got %q", cfg.AppTitle)
	}
	if cfg.AppSubtitle != "Custom Subtitle" {
		t.Errorf("Expected AppSubtitle 'Custom Subtitle', got %q", cfg.AppSubtitle)
	}
}

func TestDefaultBrandingConfig(t *testing.T) {
	cfg := Default()
	// Default branding values are set in Default()
	if cfg.Branding.AppTitle != "go-virtual" {
		t.Errorf("Expected default AppTitle 'go-virtual', got %q", cfg.Branding.AppTitle)
	}
	if cfg.Branding.AppSubtitle != "API Mock & Virtualization" {
		t.Errorf("Expected default AppSubtitle 'API Mock & Virtualization', got %q", cfg.Branding.AppSubtitle)
	}
}

func TestLoad_BrandingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
branding:
  appTitle: "Custom Title"
  appSubtitle: "Custom Sub"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Branding.AppTitle != "Custom Title" {
		t.Errorf("Expected AppTitle 'Custom Title', got %q", cfg.Branding.AppTitle)
	}
	if cfg.Branding.AppSubtitle != "Custom Sub" {
		t.Errorf("Expected AppSubtitle 'Custom Sub', got %q", cfg.Branding.AppSubtitle)
	}
}

func TestDefaultProxyConfig(t *testing.T) {
	cfg := Default()

	if cfg.Proxy.TimeoutSeconds != 30 {
		t.Errorf("Expected default proxy timeout 30, got %d", cfg.Proxy.TimeoutSeconds)
	}
	if cfg.Proxy.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be false by default")
	}
	if cfg.Proxy.MTLS.CertFile != "" {
		t.Errorf("Expected empty default CertFile, got %q", cfg.Proxy.MTLS.CertFile)
	}
	if cfg.Proxy.MTLS.KeyFile != "" {
		t.Errorf("Expected empty default KeyFile, got %q", cfg.Proxy.MTLS.KeyFile)
	}
	if cfg.Proxy.MTLS.CACertFile != "" {
		t.Errorf("Expected empty default CACertFile, got %q", cfg.Proxy.MTLS.CACertFile)
	}
}

func TestLoad_ProxyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
proxy:
  timeoutSeconds: 60
  insecureSkipVerify: true
  mtls:
    certFile: /certs/client.crt
    keyFile: /certs/client.key
    caCertFile: /certs/ca.crt
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Proxy.TimeoutSeconds != 60 {
		t.Errorf("Expected proxy timeoutSeconds 60, got %d", cfg.Proxy.TimeoutSeconds)
	}
	if !cfg.Proxy.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
	if cfg.Proxy.MTLS.CertFile != "/certs/client.crt" {
		t.Errorf("Expected certFile '/certs/client.crt', got %q", cfg.Proxy.MTLS.CertFile)
	}
	if cfg.Proxy.MTLS.KeyFile != "/certs/client.key" {
		t.Errorf("Expected keyFile '/certs/client.key', got %q", cfg.Proxy.MTLS.KeyFile)
	}
	if cfg.Proxy.MTLS.CACertFile != "/certs/ca.crt" {
		t.Errorf("Expected caCertFile '/certs/ca.crt', got %q", cfg.Proxy.MTLS.CACertFile)
	}
}

func TestMTLSConfig_Struct(t *testing.T) {
	m := MTLSConfig{
		CertFile:   "c.crt",
		KeyFile:    "c.key",
		CACertFile: "ca.crt",
	}
	if m.CertFile != "c.crt" {
		t.Errorf("CertFile mismatch: %q", m.CertFile)
	}
	if m.KeyFile != "c.key" {
		t.Errorf("KeyFile mismatch: %q", m.KeyFile)
	}
	if m.CACertFile != "ca.crt" {
		t.Errorf("CACertFile mismatch: %q", m.CACertFile)
	}
}

func TestStorageConfigNormalize(t *testing.T) {
	// Default file type is preserved.
	cfg := StorageConfig{Type: "file", Path: "/data"}
	cfg.Normalize()
	if cfg.Type != StorageTypeFile {
		t.Errorf("expected type %q, got %q", StorageTypeFile, cfg.Type)
	}
	// Mongo defaults are applied even when type is file.
	if cfg.Mongo.Database != DefaultMongoDB {
		t.Errorf("expected mongo database %q, got %q", DefaultMongoDB, cfg.Mongo.Database)
	}
	if cfg.Mongo.CollectionPrefix != DefaultMongoCollectionPrefix {
		t.Errorf("expected mongo collection prefix %q, got %q", DefaultMongoCollectionPrefix, cfg.Mongo.CollectionPrefix)
	}
	if cfg.Mongo.ConnectTimeoutSeconds != DefaultMongoConnectTimeoutSeconds {
		t.Errorf("expected connect timeout %d, got %d", DefaultMongoConnectTimeoutSeconds, cfg.Mongo.ConnectTimeoutSeconds)
	}

	// Unknown type falls back to "file".
	cfg2 := StorageConfig{Type: "unknown"}
	cfg2.Normalize()
	if cfg2.Type != StorageTypeFile {
		t.Errorf("expected fallback type %q, got %q", StorageTypeFile, cfg2.Type)
	}

	// "memory" type is preserved.
	cfg3 := StorageConfig{Type: "memory"}
	cfg3.Normalize()
	if cfg3.Type != StorageTypeMemory {
		t.Errorf("expected type %q, got %q", StorageTypeMemory, cfg3.Type)
	}
}

func TestStorageConfigNormalizeMongo(t *testing.T) {
	cfg := StorageConfig{
		Type: "mongo",
		Mongo: MongoConfig{
			URI:      "mongodb://localhost:27017",
			Database: "mydb",
		},
	}
	cfg.Normalize()

	if cfg.Type != StorageTypeMongo {
		t.Errorf("expected type %q, got %q", StorageTypeMongo, cfg.Type)
	}
	if cfg.Mongo.Database != "mydb" {
		t.Errorf("expected database %q, got %q", "mydb", cfg.Mongo.Database)
	}
	// CollectionPrefix defaults applied since it was empty.
	if cfg.Mongo.CollectionPrefix != DefaultMongoCollectionPrefix {
		t.Errorf("expected collection prefix %q, got %q", DefaultMongoCollectionPrefix, cfg.Mongo.CollectionPrefix)
	}
	if cfg.Mongo.ConnectTimeoutSeconds != DefaultMongoConnectTimeoutSeconds {
		t.Errorf("expected connect timeout %d, got %d", DefaultMongoConnectTimeoutSeconds, cfg.Mongo.ConnectTimeoutSeconds)
	}

	// Explicit values are not overwritten.
	cfg2 := StorageConfig{
		Type: "mongo",
		Mongo: MongoConfig{
			URI:                   "mongodb://host:27017",
			Database:              "customdb",
			CollectionPrefix:      "myapp_",
			ConnectTimeoutSeconds: 30,
		},
	}
	cfg2.Normalize()
	if cfg2.Mongo.Database != "customdb" {
		t.Errorf("expected database %q, got %q", "customdb", cfg2.Mongo.Database)
	}
	if cfg2.Mongo.CollectionPrefix != "myapp_" {
		t.Errorf("expected collection prefix %q, got %q", "myapp_", cfg2.Mongo.CollectionPrefix)
	}
	if cfg2.Mongo.ConnectTimeoutSeconds != 30 {
		t.Errorf("expected connect timeout 30, got %d", cfg2.Mongo.ConnectTimeoutSeconds)
	}
}
