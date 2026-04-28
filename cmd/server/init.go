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

When run interactively (TTY detected) the command guides you through key settings
with prompts and defaults. Use --no-interactive (-y) to skip all prompts and write
defaults immediately (useful in scripts and CI).

If config.yaml already exists, it will not be overwritten unless --force is used.`,
	RunE: runInit,
}

var (
	initForce         bool
	initPath          string
	initNoInteractive bool
)

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing config file")
	initCmd.Flags().StringVarP(&initPath, "path", "p", ".", "Path where to initialize (default: current directory)")
	initCmd.Flags().BoolVarP(&initNoInteractive, "no-interactive", "y", false, "Skip all prompts and use defaults (also auto-enabled when not a TTY)")
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

	// Collect config interactively or use defaults.
	p := newPrompter(initNoInteractive)
	cfg := collectInitConfig(p)

	// Confirm before writing (interactive only).
	if p.interactive {
		printInitSummary(cfg)
		if !p.PromptBool("Proceed with these settings", true) {
			fmt.Println("Aborted.")
			return nil
		}
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

	// Write config file
	configData := []byte(renderDefaultConfigYAML(cfg))
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

// collectInitConfig runs the interactive prompts (or returns defaults when
// non-interactive) and returns a fully-populated initConfigFile.
func collectInitConfig(p *prompter) initConfigFile {
	cfg := defaultInitConfig()

	// ── Server ───────────────────────────────────────────────────────────────
	p.section("Server")
	portStr := p.Prompt("Port", fmt.Sprintf("%d", cfg.Server.Port))
	port := cfg.Server.Port
	if n := parseInt(portStr); n > 0 {
		port = n
	}
	cfg.Server.Port = port
	cfg.Branding.AppTitle = p.Prompt("Branding title", cfg.Branding.AppTitle)
	cfg.Branding.AppSubtitle = p.Prompt("Branding subtitle", cfg.Branding.AppSubtitle)

	// ── Storage ──────────────────────────────────────────────────────────────
	p.section("Storage")
	cfg.Storage.Type = p.PromptSelect("Data storage type", []string{"file", "memory", "mongo"}, cfg.Storage.Type)
	if cfg.Storage.Type == "mongo" {
		cfg.Storage.Mongo.URI = p.Prompt("MongoDB URI", "mongodb://localhost:27017")
		cfg.Storage.Mongo.Database = p.Prompt("MongoDB database", "go-virtual")
	}

	// ── Session ──────────────────────────────────────────────────────────────
	p.section("Session")
	cfg.Session.StoreType = p.PromptSelect("Session store type", []string{"memory", "redis"}, cfg.Session.StoreType)
	if cfg.Session.StoreType == "redis" {
		defAddr := cfg.Session.Redis.Addr
		if defAddr == "" {
			defAddr = "localhost:6379"
		}
		cfg.Session.Redis.Addr = p.Prompt("Redis address", defAddr)
		cfg.Session.Redis.Password = p.PromptSecret("Redis password")
		defPrefix := cfg.Session.Redis.KeyPrefix
		if defPrefix == "" {
			defPrefix = "gv:session:"
		}
		cfg.Session.Redis.KeyPrefix = p.Prompt("Redis key prefix", defPrefix)
	}

	// ── AI ───────────────────────────────────────────────────────────────────
	p.section("AI")
	aiEnabled := p.PromptBool("Enable AI features", false)
	if p.interactive && !aiEnabled {
		// In interactive mode the user explicitly said no → clear the provider.
		cfg.AI.Provider = ""
	} else if aiEnabled {
		cfg.AI.Provider = p.PromptSelect("AI provider", []string{"openai", "claude"}, "openai")
		switch cfg.AI.Provider {
		case "openai":
			cfg.AI.OpenAI.APIKey = p.PromptSecret("OpenAI API key")
			cfg.AI.OpenAI.Model = p.Prompt("OpenAI model", "gpt-4o-mini")
		case "claude":
			cfg.AI.Claude.APIKey = p.PromptSecret("Claude API key")
			cfg.AI.Claude.Model = p.Prompt("Claude model", "claude-3-5-sonnet-latest")
		}
	}

	return cfg
}

func printInitSummary(cfg initConfigFile) {
	fmt.Println()
	fmt.Println("─── Configuration summary ───────────────────────────────")
	fmt.Printf("  Server port   : %d\n", cfg.Server.Port)
	fmt.Printf("  Branding      : %s — %s\n", cfg.Branding.AppTitle, cfg.Branding.AppSubtitle)
	fmt.Printf("  Storage type  : %s\n", cfg.Storage.Type)
	if cfg.Storage.Type == "mongo" {
		fmt.Printf("  MongoDB       : %s / %s\n", cfg.Storage.Mongo.URI, cfg.Storage.Mongo.Database)
	}
	fmt.Printf("  Session store : %s\n", cfg.Session.StoreType)
	if cfg.Session.StoreType == "redis" {
		fmt.Printf("  Redis         : %s (prefix: %s)\n", cfg.Session.Redis.Addr, cfg.Session.Redis.KeyPrefix)
	}
	fmt.Printf("  AI provider   : %s\n", func() string {
		if cfg.AI.Provider == "" {
			return "disabled"
		}
		return cfg.AI.Provider
	}())
	fmt.Println("─────────────────────────────────────────────────────────")
}

func parseInt(s string) int {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0
	}
	return n
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
var b strings.Builder

b.WriteString(`# Go-Virtual Configuration
# See documentation at https://github.com/prasenjit/go-virtual
#
# Tips:
# - Keep storage.path relative ("./data") if you want the project folder to stay portable.
# - Durations use Go duration syntax such as "30m", "24h", or "15s".
# - Leave API keys empty until you are ready to enable AI features.

`)

fmt.Fprintf(&b, `server:
  port: %d
  host: %q                 # Address to bind. Use "127.0.0.1" for local-only or "0.0.0.0" for all interfaces.
  tls:
    enabled: %t            # Enable HTTPS. When false, the server runs over plain HTTP.
    certFile: %q           # Path to an existing TLS certificate PEM file. Leave empty to use autoGenerate.
    keyFile: %q            # Path to an existing TLS private key PEM file. Leave empty to use autoGenerate.
    autoGenerate: %t       # Generate and reuse a local self-signed certificate when certFile/keyFile are not provided.
    storePath: %q          # Optional directory for generated certs. Default is <storage.path>/certs when empty.
  headless: %t             # Disable the admin UI/API and only serve runtime traffic from saved data.

`,
cfg.Server.Port, cfg.Server.Host,
cfg.Server.TLS.Enabled, cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile,
cfg.Server.TLS.AutoGenerate, cfg.Server.TLS.StorePath,
cfg.Server.Headless,
)

fmt.Fprintf(&b, `storage:
  type: %q                 # "file" persists to disk. "memory" uses RAM only (no persistence). "mongo" uses MongoDB.
  path: %q                 # Base directory for file storage (used when type is "file").
`,
cfg.Storage.Type, cfg.Storage.Path)

if cfg.Storage.Type == "mongo" {
fmt.Fprintf(&b, `  mongo:
    uri: %q           # MongoDB connection URI.
    database: %q      # Database name.
    collectionPrefix: "gv_"            # Prefix for all collection names.
    connectTimeoutSeconds: 10          # Connection timeout in seconds.
`,
cfg.Storage.Mongo.URI, cfg.Storage.Mongo.Database)
} else {
b.WriteString(`  # MongoDB settings — used only when type is "mongo":
  # mongo:
  #   uri: "mongodb://localhost:27017"   # MongoDB connection URI.
  #   database: "go-virtual"             # Database name.
  #   collectionPrefix: "gv_"            # Prefix for all collection names.
  #   connectTimeoutSeconds: 10          # Connection timeout in seconds.
`)
}
b.WriteString("\n")

fmt.Fprintf(&b, `tracing:
  maxTraces: %d            # Maximum number of recent traces kept in memory.
  retention: %q            # How long traces are kept before expiring.

logging:
  level: %q                # "debug", "info", "warn", or "error".
  format: %q               # "json" for machine-readable logs or "text" for human-readable logs.

branding:
  appTitle: %q             # Custom title shown in the browser tab and admin UI.
  appSubtitle: %q          # Short subtitle shown in the admin sidebar.

scripting:
  defaultTimeoutMs: %d     # Default execution timeout for each Starlark script in milliseconds.

`,
cfg.Tracing.MaxTraces, cfg.Tracing.Retention,
cfg.Logging.Level, cfg.Logging.Format,
cfg.Branding.AppTitle, cfg.Branding.AppSubtitle,
cfg.Scripting.DefaultTimeoutMs,
)

fmt.Fprintf(&b, `session:
  storeType: %q            # "memory" for single-instance. "redis" for multi-instance horizontal scaling.
  headerName: %q           # Request header used to identify and resume a session.
  inactivityTimeout: %q    # Session TTL since last activity.
  maxSessions: %d          # Hard cap on concurrent sessions (LRU eviction when exceeded).
`,
cfg.Session.StoreType, cfg.Session.HeaderName,
cfg.Session.InactivityTimeout, cfg.Session.MaxSessions,
)

if cfg.Session.StoreType == "redis" {
fmt.Fprintf(&b, `  redis:
    addr: %q               # Redis server address.
    username: %q           # Optional Redis ACL username.
    password: %q           # Optional Redis password.
    db: %d                 # Redis logical database number.
    keyPrefix: %q          # Namespace prefix for session keys.
`,
cfg.Session.Redis.Addr, cfg.Session.Redis.Username,
cfg.Session.Redis.Password, cfg.Session.Redis.DB,
cfg.Session.Redis.KeyPrefix)
} else {
fmt.Fprintf(&b, `  # Redis configuration — used only when storeType is "redis":
  redis:
    addr: %q               # Redis server address.
    username: %q           # Optional Redis ACL username.
    password: %q           # Optional Redis password.
    db: %d                 # Redis logical database number.
    keyPrefix: %q          # Namespace prefix for session keys.
`,
cfg.Session.Redis.Addr, cfg.Session.Redis.Username,
cfg.Session.Redis.Password, cfg.Session.Redis.DB,
cfg.Session.Redis.KeyPrefix)
}
b.WriteString("\n")

fmt.Fprintf(&b, `proxy:
  timeoutSeconds: %d       # Timeout for outbound backend requests in proxy mode.
  insecureSkipVerify: %t   # Skip backend TLS certificate verification (local dev only).
  mtls:
    certFile: %q           # Client certificate PEM file for backend mTLS.
    keyFile: %q            # Client private key PEM file for backend mTLS.
    caCertFile: %q         # Optional CA bundle PEM to verify the backend server certificate.

`,
cfg.Proxy.TimeoutSeconds, cfg.Proxy.InsecureSkipVerify,
cfg.Proxy.MTLS.CertFile, cfg.Proxy.MTLS.KeyFile, cfg.Proxy.MTLS.CACertFile,
)

fmt.Fprintf(&b, `ai:
  provider: %q             # Which provider powers AI features: "openai" or "claude".
  openai:
    apiKey: %q             # OpenAI API key. You can also set GOVIRTUAL_AI_OPENAI_APIKEY env var.
    model: %q              # OpenAI chat model used for AI response/script generation.
    baseUrl: %q            # Optional OpenAI-compatible base URL (Ollama, LM Studio, etc.).
  claude:
    apiKey: %q             # Anthropic API key for Claude. Also: GOVIRTUAL_AI_CLAUDE_APIKEY.
    model: %q              # Claude model used for AI response/script generation.
    baseUrl: %q            # Optional override for the Claude Messages API endpoint base URL.
    apiVersion: %q         # Anthropic API version header sent with Claude requests.

# Legacy OpenAI aliases are still supported for backward compatibility:
# ai.openaiApiKey
# ai.openaiModel
# ai.openaiBaseUrl
`,
cfg.AI.Provider,
cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.Model, cfg.AI.OpenAI.BaseURL,
cfg.AI.Claude.APIKey, cfg.AI.Claude.Model, cfg.AI.Claude.BaseURL, cfg.AI.Claude.APIVersion,
)

return b.String()
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
	Type  string           `yaml:"type"`
	Path  string           `yaml:"path"`
	Mongo initMongoConfig  `yaml:"mongo"`
}

type initMongoConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
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
