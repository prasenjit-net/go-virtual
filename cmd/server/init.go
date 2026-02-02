package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	config := map[string]interface{}{
		"server": map[string]interface{}{
			"port": 8080,
			"host": "0.0.0.0",
			"tls": map[string]interface{}{
				"enabled":      false,
				"certFile":     "",
				"keyFile":      "",
				"autoGenerate": true,
				"storePath":    "",
			},
		},
		"storage": map[string]interface{}{
			"type": "file",
			"path": "./data",
		},
		"tracing": map[string]interface{}{
			"maxTraces": 1000,
			"retention": "24h",
		},
		"logging": map[string]interface{}{
			"level":  "info",
			"format": "json",
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Add header comment
	header := `# Go-Virtual Configuration
# See documentation at https://github.com/prasenjit/go-virtual

`
	configData := []byte(header + string(data))

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
