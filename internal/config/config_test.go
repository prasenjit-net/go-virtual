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
