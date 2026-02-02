package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Tracing TracingConfig `yaml:"tracing"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port int       `yaml:"port"`
	Host string    `yaml:"host"`
	TLS  TLSConfig `yaml:"tls"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`       // Enable TLS
	CertFile     string `yaml:"certFile"`      // Path to certificate file
	KeyFile      string `yaml:"keyFile"`       // Path to private key file
	AutoGenerate bool   `yaml:"autoGenerate"`  // Auto-generate self-signed cert if not configured
	StorePath    string `yaml:"storePath"`     // Path to store auto-generated certs
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

// Default returns the default configuration
func Default() *Config {
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
			Type: "memory",
			Path: "./data",
		},
		Tracing: TracingConfig{
			MaxTraces: 1000,
			Retention: 24 * time.Hour,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
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

	return cfg, nil
}
