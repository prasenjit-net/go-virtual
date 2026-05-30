package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/prasenjit/go-virtual/internal/config"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "go-virtual",
		Short: "Go-Virtual - API proxy/mock service for OpenAPI 3 specs",
		Long: `Go-Virtual is an API proxy/mock service that virtualizes OpenAPI 3 specifications.
It allows configuring custom responses based on request conditions, with support for 
templating, tracing, and statistics.`,
	}
)

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ./config.yaml)")

	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(initCmd)
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}

		// Search config in current directory
		viper.AddConfigPath(cwd)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("GOVIRTUAL")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// setDefaults sets the default configuration values
func setDefaults() {
	// Get current working directory for default data path
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	defaultDataPath := filepath.Join(cwd, "data")

	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.tls.enabled", false)
	viper.SetDefault("server.tls.certFile", "")
	viper.SetDefault("server.tls.keyFile", "")
	viper.SetDefault("server.tls.autoGenerate", true)
	viper.SetDefault("server.tls.storePath", "")

	// Storage defaults
	viper.SetDefault("storage.type", "file")
	viper.SetDefault("storage.path", defaultDataPath)
	viper.SetDefault("storage.mongo.uri", "")
	viper.SetDefault("storage.mongo.database", config.DefaultMongoDB)
	viper.SetDefault("storage.mongo.collectionPrefix", config.DefaultMongoCollectionPrefix)
	viper.SetDefault("storage.mongo.connectTimeoutSeconds", config.DefaultMongoConnectTimeoutSeconds)

	// Tracing defaults
	viper.SetDefault("tracing.maxTraces", 1000)
	viper.SetDefault("tracing.retention", "24h")

	// Session defaults
	viper.SetDefault("session.storeType", config.SessionStoreMemory)
	viper.SetDefault("session.headerName", "X-Virtual-Session-Id")
	viper.SetDefault("session.inactivityTimeout", "30m")
	viper.SetDefault("session.maxSessions", 10000)
	viper.SetDefault("session.redis.addr", config.DefaultRedisAddr)
	viper.SetDefault("session.redis.username", "")
	viper.SetDefault("session.redis.password", "")
	viper.SetDefault("session.redis.db", 0)
	viper.SetDefault("session.redis.keyPrefix", config.DefaultRedisKeyPrefix)

	// Logging defaults
	viper.SetDefault("logging.level", config.LogLevelInfo)
	viper.SetDefault("logging.format", config.LogFormatJSON)

	// AI defaults
	viper.SetDefault("ai.provider", config.AIProviderOpenAI)
	viper.SetDefault("ai.httpProxy", "")
	viper.SetDefault("ai.openai.apiKey", "")
	viper.SetDefault("ai.openai.model", config.DefaultOpenAIModel)
	viper.SetDefault("ai.openai.baseUrl", "")
	viper.SetDefault("ai.claude.apiKey", "")
	viper.SetDefault("ai.claude.model", config.DefaultClaudeModel)
	viper.SetDefault("ai.claude.baseUrl", "")
	viper.SetDefault("ai.claude.apiVersion", config.DefaultClaudeAPIVersion)
	viper.SetDefault("ai.copilot.oauthToken", "")
	viper.SetDefault("ai.copilot.model", config.DefaultCopilotModel)
	viper.SetDefault("ai.copilot.baseUrl", "")
	viper.SetDefault("ai.openaiApiKey", "")
	viper.SetDefault("ai.openaiModel", config.DefaultOpenAIModel)
	viper.SetDefault("ai.openaiBaseUrl", "")
}
