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
	LogLevelDebug           = "debug"
	LogLevelInfo            = "info"
	LogLevelWarn            = "warn"
	LogLevelError           = "error"
	LogFormatJSON           = "json"
	LogFormatText           = "text"
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

// StorageConfig holds storage configuration
type StorageConfig struct {
	Type string `yaml:"type"` // "memory" or "file"
	Path string `yaml:"path"` // Path for file storage
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
	// HeaderName is the HTTP header used to identify the session. Defaults to "X-Virtual-Session-Id".
	HeaderName string `yaml:"headerName"`
	// InactivityTimeout is how long a session survives without activity. Defaults to 30 minutes.
	InactivityTimeout time.Duration `yaml:"inactivityTimeout"`
	// MaxSessions is a hard cap on concurrent sessions. When exceeded the
	// least-recently-active session is evicted. Defaults to 10000.
	MaxSessions int `yaml:"maxSessions"`
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
			HeaderName:        "X-Virtual-Session-Id",
			InactivityTimeout: 30 * time.Minute,
			MaxSessions:       10000,
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
