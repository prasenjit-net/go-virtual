package main

import (
	"os"
	"path/filepath"
	"testing"

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
