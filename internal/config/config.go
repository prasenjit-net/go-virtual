package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	AIProviderOpenAI        = "openai"
	AIProviderClaude        = "claude"
	DefaultOpenAIModel      = "gpt-4o-mini"
	DefaultClaudeModel      = "claude-sonnet-4-6"
	DefaultClaudeAPIVersion = "2023-06-01"
	SessionStoreMemory      = "memory"
	SessionStoreRedis       = "redis"
	DefaultRedisAddr        = "127.0.0.1:6379"
	DefaultRedisKeyPrefix   = "go-virtual:sessions"
	LogLevelDebug           = "debug"
	LogLevelInfo            = "info"
	LogLevelWarn            = "warn"
	LogLevelError           = "error"
	LogFormatJSON           = "json"
	LogFormatText           = "text"
	StorageTypeFile         = "file"
	StorageTypeMongo        = "mongo"
	StorageTypeMemory       = "memory"
	DefaultMongoDB                    = "go-virtual"
	DefaultMongoCollectionPrefix      = "gv_"
	DefaultMongoConnectTimeoutSeconds = 10
	DefaultMongoStartupRetrySeconds   = 60
)

// Config holds the application configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Storage   StorageConfig   `yaml:"storage"`
	Tracing   TracingConfig   `yaml:"tracing"`
	Logging   LoggingConfig   `yaml:"logging"`
	Branding  BrandingConfig  `yaml:"branding"`
	Scripting ScriptingConfig `yaml:"scripting"`
	Session   SessionConfig   `yaml:"session"`
	Proxy     ProxyConfig     `yaml:"proxy"`
	AI        AIConfig        `yaml:"ai"`
}

// AIConfig holds configuration for AI-powered response generation.
type AIConfig struct {
	// Provider selects which provider powers AI features. Supported values:
	// "openai" and "claude". Defaults to "openai".
	Provider string `yaml:"provider"`
	// OpenAI holds OpenAI-compatible provider configuration.
	OpenAI OpenAIConfig `yaml:"openai"`
	// Claude holds Anthropic Claude provider configuration.
	Claude ClaudeConfig `yaml:"claude"`
	// Legacy OpenAI aliases kept for backwards compatibility with existing
	// config files and environment variable names.
	OpenAIAPIKey  string `yaml:"openaiApiKey"`
	OpenAIModel   string `yaml:"openaiModel"`
	OpenAIBaseURL string `yaml:"openaiBaseUrl"`
}

// OpenAIConfig holds settings for OpenAI-compatible providers.
type OpenAIConfig struct {
	APIKey  string `yaml:"apiKey"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"baseUrl"`
}

// ClaudeConfig holds settings for Anthropic Claude.
type ClaudeConfig struct {
	APIKey     string `yaml:"apiKey"`
	Model      string `yaml:"model"`
	BaseURL    string `yaml:"baseUrl"`
	APIVersion string `yaml:"apiVersion"`
}

// Normalize applies defaults and migrates legacy aliases to the provider-aware shape.
func (c *AIConfig) Normalize() {
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	if c.Provider == "" {
		c.Provider = AIProviderOpenAI
	}

	if strings.TrimSpace(c.OpenAI.APIKey) == "" {
		c.OpenAI.APIKey = strings.TrimSpace(c.OpenAIAPIKey)
	}
	if strings.TrimSpace(c.OpenAIModel) != "" && (strings.TrimSpace(c.OpenAI.Model) == "" || c.OpenAI.Model == DefaultOpenAIModel) {
		c.OpenAI.Model = strings.TrimSpace(c.OpenAIModel)
	}
	if strings.TrimSpace(c.OpenAI.Model) == "" {
		c.OpenAI.Model = strings.TrimSpace(c.OpenAIModel)
	}
	if strings.TrimSpace(c.OpenAI.BaseURL) == "" {
		c.OpenAI.BaseURL = strings.TrimSpace(c.OpenAIBaseURL)
	}
	if strings.TrimSpace(c.OpenAI.Model) == "" {
		c.OpenAI.Model = DefaultOpenAIModel
	}

	if strings.TrimSpace(c.Claude.Model) == "" {
		c.Claude.Model = DefaultClaudeModel
	}
	if strings.TrimSpace(c.Claude.APIVersion) == "" {
		c.Claude.APIVersion = DefaultClaudeAPIVersion
	}
}

// BrandingConfig holds UI branding configuration
type BrandingConfig struct {
	// AppTitle overrides the application name shown in the sidebar and browser tab.
	// Defaults to "go-virtual" when empty.
	AppTitle string `yaml:"appTitle"`
	// AppSubtitle is a short tagline shown below the title in the sidebar footer.
	AppSubtitle string `yaml:"appSubtitle"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port     int       `yaml:"port"`
	Host     string    `yaml:"host"`
	TLS      TLSConfig `yaml:"tls"`
	Headless bool      `yaml:"headless"` // Disable admin API and UI; serve only proxy responses from saved data
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`      // Enable TLS
	CertFile     string `yaml:"certFile"`     // Path to certificate file
	KeyFile      string `yaml:"keyFile"`      // Path to private key file
	AutoGenerate bool   `yaml:"autoGenerate"` // Auto-generate self-signed cert if not configured
	StorePath    string `yaml:"storePath"`    // Path to store auto-generated certs
}

// MongoSyncMode controls how this instance detects external changes made by
// other instances sharing the same MongoDB database.
type MongoSyncMode string

const (
	// MongoSyncModeAuto tries change streams first and falls back to polling
	// when the deployment does not support them (standalone mongod).
	MongoSyncModeAuto MongoSyncMode = "auto"
	// MongoSyncModeChangeStream forces change-stream mode; returns an error on
	// deployments that do not support it.
	MongoSyncModeChangeStream MongoSyncMode = "change_stream"
	// MongoSyncModePolling uses periodic polling regardless of deployment type.
	MongoSyncModePolling MongoSyncMode = "polling"
	// MongoSyncModeOff disables cross-instance synchronisation entirely.
	// Only suitable for single-instance deployments.
	MongoSyncModeOff MongoSyncMode = "off"

	DefaultMongoSyncPollIntervalSeconds = 10
)

// MongoSyncConfig holds settings for cross-instance synchronisation when
// MongoDB is the storage backend.
type MongoSyncConfig struct {
	// Mode controls the synchronisation strategy. Defaults to "auto".
	Mode MongoSyncMode `yaml:"mode"`
	// PollIntervalSeconds is the polling cadence used in "polling" and "auto"
	// (fallback) modes. Defaults to 10.
	PollIntervalSeconds int `yaml:"pollIntervalSeconds"`
}

// Normalize applies defaults.
func (c *MongoSyncConfig) Normalize() {
	switch c.Mode {
	case MongoSyncModeChangeStream, MongoSyncModePolling, MongoSyncModeOff:
		// valid — keep as-is
	default:
		c.Mode = MongoSyncModeAuto
	}
	if c.PollIntervalSeconds <= 0 {
		c.PollIntervalSeconds = DefaultMongoSyncPollIntervalSeconds
	}
}

// MongoConfig holds settings for MongoDB-backed storage.
type MongoConfig struct {
	URI                   string          `yaml:"uri"`
	Database              string          `yaml:"database"`
	CollectionPrefix      string          `yaml:"collectionPrefix"`
	ConnectTimeoutSeconds int             `yaml:"connectTimeoutSeconds"`
	// StartupRetrySeconds is the total time budget go-virtual will spend
	// retrying the initial MongoDB ping. Useful in Docker Swarm / Kubernetes
	// where the replica set may not be ready when this container first starts.
	// 0 means a single attempt only (no retry). Default: 60.
	StartupRetrySeconds int `yaml:"startupRetrySeconds"`
	// Sync controls cross-instance route and store synchronisation.
	Sync MongoSyncConfig `yaml:"sync"`
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	Type  string      `yaml:"type"`  // "file", "memory", or "mongo"
	Path  string      `yaml:"path"`  // Path for file storage
	Mongo MongoConfig `yaml:"mongo"` // MongoDB settings (used when Type is "mongo")
}

// Normalize applies defaults and normalises the storage configuration.
func (c *StorageConfig) Normalize() {
	t := strings.ToLower(strings.TrimSpace(c.Type))
	switch t {
	case StorageTypeFile, StorageTypeMongo, StorageTypeMemory:
		c.Type = t
	default:
		c.Type = StorageTypeFile
	}

	if c.Mongo.Database == "" {
		c.Mongo.Database = DefaultMongoDB
	}
	if c.Mongo.CollectionPrefix == "" {
		c.Mongo.CollectionPrefix = DefaultMongoCollectionPrefix
	}
	if c.Mongo.ConnectTimeoutSeconds <= 0 {
		c.Mongo.ConnectTimeoutSeconds = DefaultMongoConnectTimeoutSeconds
	}
	if c.Mongo.StartupRetrySeconds < 0 {
		c.Mongo.StartupRetrySeconds = 0
	}
	if c.Mongo.StartupRetrySeconds == 0 {
		c.Mongo.StartupRetrySeconds = DefaultMongoStartupRetrySeconds
	}
	c.Mongo.Sync.Normalize()
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	MaxTraces int           `yaml:"maxTraces"`
	Retention time.Duration `yaml:"retention"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Normalize applies supported defaults for logging configuration.
func (c *LoggingConfig) Normalize() {
	level := strings.ToLower(strings.TrimSpace(c.Level))
	switch level {
	case "", LogLevelInfo:
		c.Level = LogLevelInfo
	case LogLevelDebug, LogLevelWarn, LogLevelError:
		c.Level = level
	default:
		c.Level = LogLevelInfo
	}

	format := strings.ToLower(strings.TrimSpace(c.Format))
	switch format {
	case "", LogFormatJSON:
		c.Format = LogFormatJSON
	case LogFormatText:
		c.Format = format
	default:
		c.Format = LogFormatJSON
	}
}

// SessionConfig controls per-request session behaviour.
type SessionConfig struct {
	// StoreType controls where sessions are persisted. Supported values are
	// "memory" and "redis". Defaults to "memory".
	StoreType string `yaml:"storeType"`
	// HeaderName is the HTTP header used to identify the session. Defaults to "X-Virtual-Session-Id".
	HeaderName string `yaml:"headerName"`
	// InactivityTimeout is how long a session survives without activity. Defaults to 30 minutes.
	InactivityTimeout time.Duration `yaml:"inactivityTimeout"`
	// MaxSessions is a hard cap on concurrent sessions. When exceeded the
	// least-recently-active session is evicted. Defaults to 10000.
	MaxSessions int `yaml:"maxSessions"`
	// Redis holds Redis-specific settings used when StoreType is "redis".
	Redis RedisSessionConfig `yaml:"redis"`
}

// RedisSessionConfig holds settings for Redis-backed session storage.
type RedisSessionConfig struct {
	Addr      string `yaml:"addr"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	DB        int    `yaml:"db"`
	KeyPrefix string `yaml:"keyPrefix"`
}

// Normalize applies supported defaults for session configuration.
func (c *SessionConfig) Normalize() {
	storeType := strings.ToLower(strings.TrimSpace(c.StoreType))
	switch storeType {
	case "", SessionStoreMemory:
		c.StoreType = SessionStoreMemory
	case SessionStoreRedis:
		c.StoreType = storeType
	default:
		c.StoreType = SessionStoreMemory
	}

	c.HeaderName = strings.TrimSpace(c.HeaderName)
	if c.HeaderName == "" {
		c.HeaderName = "X-Virtual-Session-Id"
	}
	if c.InactivityTimeout <= 0 {
		c.InactivityTimeout = 30 * time.Minute
	}
	if c.MaxSessions <= 0 {
		c.MaxSessions = 10000
	}
	c.Redis.Addr = strings.TrimSpace(c.Redis.Addr)
	if c.Redis.Addr == "" {
		c.Redis.Addr = DefaultRedisAddr
	}
	c.Redis.Username = strings.TrimSpace(c.Redis.Username)
	c.Redis.Password = strings.TrimSpace(c.Redis.Password)
	if c.Redis.DB < 0 {
		c.Redis.DB = 0
	}
	c.Redis.KeyPrefix = strings.TrimSpace(c.Redis.KeyPrefix)
	if c.Redis.KeyPrefix == "" {
		c.Redis.KeyPrefix = DefaultRedisKeyPrefix
	}
}

// ScriptingConfig holds Starlark scripting configuration
type ScriptingConfig struct {
	// DefaultTimeoutMs is the maximum wall-clock execution time per script in
	// milliseconds. Individual scripts can override this with their Timeout field.
	DefaultTimeoutMs int `yaml:"defaultTimeoutMs"`
}

// ProxyConfig holds configuration for outbound HTTP calls made in proxy/recording mode.
type ProxyConfig struct {
	// TimeoutSeconds is the HTTP client timeout for backend requests. Defaults to 30.
	TimeoutSeconds int `yaml:"timeoutSeconds"`
	// InsecureSkipVerify disables TLS server certificate verification.
	// Useful for backends with self-signed certificates. Defaults to false.
	InsecureSkipVerify bool `yaml:"insecureSkipVerify"`
	// MTLS configures mutual TLS — presenting a client certificate to the backend.
	MTLS MTLSConfig `yaml:"mtls"`
}

// MTLSConfig holds client certificate settings for mutual TLS.
type MTLSConfig struct {
	// CertFile is the path to the PEM-encoded client certificate.
	CertFile string `yaml:"certFile"`
	// KeyFile is the path to the PEM-encoded client private key.
	KeyFile string `yaml:"keyFile"`
	// CACertFile is an optional path to a PEM-encoded CA certificate used to
	// verify the backend server's certificate. When empty the system CA pool is used.
	CACertFile string `yaml:"caCertFile"`
}

// Default returns the default configuration
func Default() *Config {
	// Get current working directory for default data path
	// Use filepath.Join for cross-platform compatibility (Windows, Linux, macOS)
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	defaultDataPath := filepath.Join(cwd, "data")

	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
			TLS: TLSConfig{
				Enabled:      false,
				AutoGenerate: true,
				StorePath:    "", // Empty means use storage.path/certs
			},
		},
		Storage: StorageConfig{
			Type: "file",
			Path: defaultDataPath,
			Mongo: MongoConfig{
				Database:              DefaultMongoDB,
				CollectionPrefix:      DefaultMongoCollectionPrefix,
				ConnectTimeoutSeconds: DefaultMongoConnectTimeoutSeconds,
			},
		},
		Tracing: TracingConfig{
			MaxTraces: 1000,
			Retention: 24 * time.Hour,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Branding: BrandingConfig{
			AppTitle:    "go-virtual",
			AppSubtitle: "API Mock & Virtualization",
		},
		Scripting: ScriptingConfig{
			DefaultTimeoutMs: 100,
		},
		Session: SessionConfig{
			StoreType:         SessionStoreMemory,
			HeaderName:        "X-Virtual-Session-Id",
			InactivityTimeout: 30 * time.Minute,
			MaxSessions:       10000,
			Redis: RedisSessionConfig{
				Addr:      DefaultRedisAddr,
				DB:        0,
				KeyPrefix: DefaultRedisKeyPrefix,
			},
		},
		Proxy: ProxyConfig{
			TimeoutSeconds:     30,
			InsecureSkipVerify: false,
		},
		AI: AIConfig{
			Provider: AIProviderOpenAI,
			OpenAI: OpenAIConfig{
				Model: DefaultOpenAIModel,
			},
			Claude: ClaudeConfig{
				Model:      DefaultClaudeModel,
				APIVersion: DefaultClaudeAPIVersion,
			},
		},
	}
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.AI.Normalize()
	cfg.Logging.Normalize()
	cfg.Session.Normalize()
	cfg.Storage.Normalize()

	// Convert relative storage path to absolute using current working directory
	// This ensures consistent behavior across all platforms
	if cfg.Storage.Path != "" && !filepath.IsAbs(cfg.Storage.Path) {
		cwd, err := os.Getwd()
		if err == nil {
			cfg.Storage.Path = filepath.Join(cwd, cfg.Storage.Path)
		}
	}

	return cfg, nil
}
