package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize go-virtual with default configuration and directory structure",
	Long: `Creates the default configuration file (config.yaml) and data directory structure.

This command will:
  - Create config.yaml with default settings
  - Create data/ directory for file storage
  - Create data/specs/ directory for OpenAPI specs
  - Create data/responses/ directory for response configurations

If config.yaml already exists, it will not be overwritten unless --force is used.`,
	RunE: runInit,
}

var (
	initForce bool
	initPath  string
)

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing config file")
	initCmd.Flags().StringVarP(&initPath, "path", "p", ".", "Path where to initialize (default: current directory)")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Resolve path to absolute
	absPath, err := filepath.Abs(initPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	configFile := filepath.Join(absPath, "config.yaml")
	dataDir := filepath.Join(absPath, "data")

	// Check if config already exists
	if _, err := os.Stat(configFile); err == nil && !initForce {
		return fmt.Errorf("config.yaml already exists. Use --force to overwrite")
	}

	// Create directory structure
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "specs"),
		filepath.Join(dataDir, "responses"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		fmt.Printf("Created directory: %s\n", dir)
	}

	// Create default config
	configData := []byte(renderDefaultConfigYAML(defaultInitConfig()))

	// Write config file
	if err := os.WriteFile(configFile, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	fmt.Printf("Created config file: %s\n", configFile)

	fmt.Println()
	fmt.Println("Initialization complete! You can now start the server with:")
	fmt.Println()
	fmt.Printf("  cd %s\n", absPath)
	fmt.Println("  go-virtual serve")
	fmt.Println()

	return nil
}

func defaultInitConfig() initConfigFile {
	cfg := config.Default()

	return initConfigFile{
		Server: initServerConfig{
			Port:     cfg.Server.Port,
			Host:     cfg.Server.Host,
			Headless: cfg.Server.Headless,
			TLS: initTLSConfig{
				Enabled:      cfg.Server.TLS.Enabled,
				CertFile:     cfg.Server.TLS.CertFile,
				KeyFile:      cfg.Server.TLS.KeyFile,
				AutoGenerate: cfg.Server.TLS.AutoGenerate,
				StorePath:    cfg.Server.TLS.StorePath,
			},
		},
		Storage: initStorageConfig{
			Type: cfg.Storage.Type,
			Path: "./data",
		},
		Tracing: initTracingConfig{
			MaxTraces: cfg.Tracing.MaxTraces,
			Retention: formatDuration(cfg.Tracing.Retention),
		},
		Logging: initLoggingConfig{
			Level:  cfg.Logging.Level,
			Format: cfg.Logging.Format,
		},
		Branding: initBrandingConfig{
			AppTitle:    cfg.Branding.AppTitle,
			AppSubtitle: cfg.Branding.AppSubtitle,
		},
		Scripting: initScriptingConfig{
			DefaultTimeoutMs: cfg.Scripting.DefaultTimeoutMs,
		},
		Session: initSessionConfig{
			StoreType:         cfg.Session.StoreType,
			HeaderName:        cfg.Session.HeaderName,
			InactivityTimeout: formatDuration(cfg.Session.InactivityTimeout),
			MaxSessions:       cfg.Session.MaxSessions,
			Redis: initSessionRedisConfig{
				Addr:      cfg.Session.Redis.Addr,
				Username:  cfg.Session.Redis.Username,
				Password:  cfg.Session.Redis.Password,
				DB:        cfg.Session.Redis.DB,
				KeyPrefix: cfg.Session.Redis.KeyPrefix,
			},
		},
		Proxy: initProxyConfig{
			TimeoutSeconds:     cfg.Proxy.TimeoutSeconds,
			InsecureSkipVerify: cfg.Proxy.InsecureSkipVerify,
			MTLS: initMTLSConfig{
				CertFile:   cfg.Proxy.MTLS.CertFile,
				KeyFile:    cfg.Proxy.MTLS.KeyFile,
				CACertFile: cfg.Proxy.MTLS.CACertFile,
			},
		},
		AI: initAIConfig{
			Provider: cfg.AI.Provider,
			OpenAI: initOpenAIConfig{
				APIKey:  cfg.AI.OpenAI.APIKey,
				Model:   cfg.AI.OpenAI.Model,
				BaseURL: cfg.AI.OpenAI.BaseURL,
			},
			Claude: initClaudeConfig{
				APIKey:     cfg.AI.Claude.APIKey,
				Model:      cfg.AI.Claude.Model,
				BaseURL:    cfg.AI.Claude.BaseURL,
				APIVersion: cfg.AI.Claude.APIVersion,
			},
		},
	}
}

func renderDefaultConfigYAML(cfg initConfigFile) string {
	return fmt.Sprintf(`# Go-Virtual Configuration
# See documentation at https://github.com/prasenjit/go-virtual
#
# Tips:
# - Keep storage.path relative ("./data") if you want the project folder to stay portable.
# - Durations use Go duration syntax such as "30m", "24h", or "15s".
# - Leave API keys empty until you are ready to enable AI features.

server:
  port: %d
  host: %q                 # Address to bind. Use "127.0.0.1" for local-only access or "0.0.0.0" to listen on all interfaces.
  tls:
    enabled: %t            # Enable HTTPS. When false, the server runs over plain HTTP.
    certFile: %q           # Path to an existing TLS certificate PEM file. Leave empty to use autoGenerate.
    keyFile: %q            # Path to an existing TLS private key PEM file. Leave empty to use autoGenerate.
    autoGenerate: %t       # Generate and reuse a local self-signed certificate when certFile/keyFile are not provided.
    storePath: %q          # Optional directory for generated certs. Default is <storage.path>/certs when empty.
  headless: %t             # Disable the admin UI/API and only serve runtime traffic from saved data.

storage:
  type: %q                 # "file" persists specs/responses to disk. "memory" keeps everything in RAM (no persistence). "mongo" uses MongoDB.
  path: %q                 # Base directory for persisted specs, responses, traces, and other app data (used when type is "file").
  # MongoDB settings — used only when type is "mongo":
  # mongo:
  #   uri: "mongodb://localhost:27017"   # MongoDB connection URI.
  #   database: "go-virtual"             # Database name.
  #   collectionPrefix: "gv_"            # Prefix for all collection names.
  #   connectTimeoutSeconds: 10          # Connection timeout in seconds.

tracing:
  maxTraces: %d            # Maximum number of recent traces kept in memory for inspection in the UI/API.
  retention: %q            # How long traces are kept before expiring.

logging:
  level: %q                # "debug", "info", "warn", or "error".
  format: %q               # "json" for machine-readable logs or "text" for simpler human-readable logs.

branding:
  appTitle: %q             # Custom title shown in the browser tab and admin UI.
  appSubtitle: %q          # Short subtitle shown in the admin sidebar.

scripting:
  defaultTimeoutMs: %d     # Default execution timeout for each Starlark script in milliseconds.

session:
  storeType: %q            # "memory" keeps sessions inside one process. "redis" shares sessions across instances for horizontal scaling.
  headerName: %q           # Request header used to identify and resume a session.
  inactivityTimeout: %q    # Session TTL since last activity.
  maxSessions: %d          # Hard cap on concurrent sessions before least-recently-used sessions are evicted. For Redis this is best-effort across instances.
  # Redis configuration example used when session.storeType is "redis":
  redis:
    addr: %q               # Redis server address used when session.storeType is "redis".
    username: %q           # Optional Redis ACL username.
    password: %q           # Optional Redis password.
    db: %d                 # Redis logical database number.
    keyPrefix: %q          # Namespace prefix for session keys so invalidate-all only touches this app's sessions.

proxy:
  timeoutSeconds: %d       # Timeout for outbound backend requests in proxy mode.
  insecureSkipVerify: %t   # Skip backend TLS certificate verification. Useful for local self-signed certs only.
  mtls:
    certFile: %q           # Client certificate PEM file for backend mTLS.
    keyFile: %q            # Client private key PEM file for backend mTLS.
    caCertFile: %q         # Optional CA bundle PEM file used to verify the backend server certificate.

ai:
  provider: %q             # Which provider powers AI features: "openai" or "claude".
  openai:
    apiKey: %q             # OpenAI API key. You can also use GOVIRTUAL_AI_OPENAI_APIKEY or legacy GOVIRTUAL_AI_OPENAIAPIKEY.
    model: %q              # OpenAI chat model used for AI response/script generation.
    baseUrl: %q            # Optional OpenAI-compatible base URL. Examples: Ollama http://localhost:11434/v1, LM Studio http://localhost:1234/v1
  claude:
    apiKey: %q             # Anthropic API key for Claude.
    model: %q              # Claude model used for AI response/script generation.
    baseUrl: %q            # Optional override for the Claude Messages API endpoint base URL.
    apiVersion: %q         # Anthropic API version header sent with Claude requests.

# Legacy OpenAI aliases are still supported for backward compatibility:
# ai.openaiApiKey
# ai.openaiModel
# ai.openaiBaseUrl
`, cfg.Server.Port, cfg.Server.Host, cfg.Server.TLS.Enabled, cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile, cfg.Server.TLS.AutoGenerate, cfg.Server.TLS.StorePath, cfg.Server.Headless, cfg.Storage.Type, cfg.Storage.Path, cfg.Tracing.MaxTraces, cfg.Tracing.Retention, cfg.Logging.Level, cfg.Logging.Format, cfg.Branding.AppTitle, cfg.Branding.AppSubtitle, cfg.Scripting.DefaultTimeoutMs, cfg.Session.StoreType, cfg.Session.HeaderName, cfg.Session.InactivityTimeout, cfg.Session.MaxSessions, cfg.Session.Redis.Addr, cfg.Session.Redis.Username, cfg.Session.Redis.Password, cfg.Session.Redis.DB, cfg.Session.Redis.KeyPrefix, cfg.Proxy.TimeoutSeconds, cfg.Proxy.InsecureSkipVerify, cfg.Proxy.MTLS.CertFile, cfg.Proxy.MTLS.KeyFile, cfg.Proxy.MTLS.CACertFile, cfg.AI.Provider, cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.Model, cfg.AI.OpenAI.BaseURL, cfg.AI.Claude.APIKey, cfg.AI.Claude.Model, cfg.AI.Claude.BaseURL, cfg.AI.Claude.APIVersion)
}

func formatDuration(d time.Duration) string {
	s := d.String()
	if s == "0s" {
		return s
	}
	if strings.Contains(s, "h") && strings.HasSuffix(s, "0m0s") {
		return strings.TrimSuffix(s, "0m0s")
	}
	if strings.HasSuffix(s, "0s") {
		return strings.TrimSuffix(s, "0s")
	}
	return s
}

type initConfigFile struct {
	Server    initServerConfig    `yaml:"server"`
	Storage   initStorageConfig   `yaml:"storage"`
	Tracing   initTracingConfig   `yaml:"tracing"`
	Logging   initLoggingConfig   `yaml:"logging"`
	Branding  initBrandingConfig  `yaml:"branding"`
	Scripting initScriptingConfig `yaml:"scripting"`
	Session   initSessionConfig   `yaml:"session"`
	Proxy     initProxyConfig     `yaml:"proxy"`
	AI        initAIConfig        `yaml:"ai"`
}

type initServerConfig struct {
	Port     int           `yaml:"port"`
	Host     string        `yaml:"host"`
	TLS      initTLSConfig `yaml:"tls"`
	Headless bool          `yaml:"headless"`
}

type initTLSConfig struct {
	Enabled      bool   `yaml:"enabled"`
	CertFile     string `yaml:"certFile"`
	KeyFile      string `yaml:"keyFile"`
	AutoGenerate bool   `yaml:"autoGenerate"`
	StorePath    string `yaml:"storePath"`
}

type initStorageConfig struct {
	Type string `yaml:"type"`
	Path string `yaml:"path"`
}

type initTracingConfig struct {
	MaxTraces int    `yaml:"maxTraces"`
	Retention string `yaml:"retention"`
}

type initLoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type initBrandingConfig struct {
	AppTitle    string `yaml:"appTitle"`
	AppSubtitle string `yaml:"appSubtitle"`
}

type initScriptingConfig struct {
	DefaultTimeoutMs int `yaml:"defaultTimeoutMs"`
}

type initSessionConfig struct {
	StoreType         string                 `yaml:"storeType"`
	HeaderName        string                 `yaml:"headerName"`
	InactivityTimeout string                 `yaml:"inactivityTimeout"`
	MaxSessions       int                    `yaml:"maxSessions"`
	Redis             initSessionRedisConfig `yaml:"redis"`
}

type initSessionRedisConfig struct {
	Addr      string `yaml:"addr"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	DB        int    `yaml:"db"`
	KeyPrefix string `yaml:"keyPrefix"`
}

type initProxyConfig struct {
	TimeoutSeconds     int            `yaml:"timeoutSeconds"`
	InsecureSkipVerify bool           `yaml:"insecureSkipVerify"`
	MTLS               initMTLSConfig `yaml:"mtls"`
}

type initMTLSConfig struct {
	CertFile   string `yaml:"certFile"`
	KeyFile    string `yaml:"keyFile"`
	CACertFile string `yaml:"caCertFile"`
}

type initAIConfig struct {
	Provider string           `yaml:"provider"`
	OpenAI   initOpenAIConfig `yaml:"openai"`
	Claude   initClaudeConfig `yaml:"claude"`
}

type initOpenAIConfig struct {
	APIKey  string `yaml:"apiKey"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"baseUrl"`
}

type initClaudeConfig struct {
	APIKey     string `yaml:"apiKey"`
	Model      string `yaml:"model"`
	BaseURL    string `yaml:"baseUrl"`
	APIVersion string `yaml:"apiVersion"`
}
