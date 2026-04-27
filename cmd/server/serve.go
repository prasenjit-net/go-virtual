package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	govirtual "github.com/prasenjit/go-virtual"
	"github.com/prasenjit/go-virtual/internal/ai"
	"github.com/prasenjit/go-virtual/internal/api"
	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/logging"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	gvstore "github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/tlsutil"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Go-Virtual server",
	Long: `Starts the Go-Virtual API proxy/mock server.

The server will:
  - Load OpenAPI specs from the data directory
  - Serve the Admin UI at /_ui/
  - Expose the Admin API at /_api/
  - Proxy requests matching loaded specs

Configuration is loaded from config.yaml in the current directory,
or specify a custom config file with the --config flag.`,
	RunE: runServe,
}

var (
	devMode      bool
	portFlag     int
	tlsFlag      bool
	headlessFlag bool
)

func init() {
	serveCmd.Flags().BoolVar(&devMode, "dev", false, "Enable development mode (serve UI from filesystem)")
	serveCmd.Flags().IntVarP(&portFlag, "port", "p", 0, "Override server port")
	serveCmd.Flags().BoolVar(&tlsFlag, "tls", false, "Enable TLS (overrides config)")
	serveCmd.Flags().BoolVar(&headlessFlag, "headless", false, "Headless mode: disable admin API and UI, serve proxy only")

	// Bind flags to viper
	viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("server.tls.enabled", serveCmd.Flags().Lookup("tls"))
	viper.BindPFlag("server.headless", serveCmd.Flags().Lookup("headless"))
}

func runServe(cmd *cobra.Command, args []string) error {
	logger, loggingWarnings := logging.Setup(config.LoggingConfig{
		Level:  viper.GetString("logging.level"),
		Format: viper.GetString("logging.format"),
	})
	serverLog := logger.With("component", "server")
	for _, warning := range loggingWarnings {
		serverLog.Warn("Logging configuration fallback applied", "event", "logging_config_warning", "warning", warning)
	}

	// Get configuration values
	port := viper.GetInt("server.port")
	host := viper.GetString("server.host")
	storageType := viper.GetString("storage.type")
	storagePath := viper.GetString("storage.path")
	maxTraces := viper.GetInt("tracing.maxTraces")
	tlsEnabled := viper.GetBool("server.tls.enabled")

	// Override port if flag was explicitly set
	if portFlag > 0 {
		port = portFlag
	}

	// Override TLS if flag was explicitly set
	if tlsFlag {
		tlsEnabled = true
	}

	// Resolve relative storage path to absolute
	if storagePath != "" && !filepath.IsAbs(storagePath) {
		cwd, err := os.Getwd()
		if err == nil {
			storagePath = filepath.Join(cwd, storagePath)
		}
	}

	// Log the data path being used
	serverLog.Info("Using data directory", "event", "startup_data_directory", "path", storagePath, "storage_type", storageType)

	// Initialize storage
	var store storage.Storage
	var err error
	storageMongoURI := viper.GetString("storage.mongo.uri")
	storageMongoDatabase := viper.GetString("storage.mongo.database")
	storageMongoCollectionPrefix := viper.GetString("storage.mongo.collectionPrefix")
	storageMongoConnectTimeout := viper.GetInt("storage.mongo.connectTimeoutSeconds")
	if storageMongoDatabase == "" {
		storageMongoDatabase = config.DefaultMongoDB
	}
	if storageMongoCollectionPrefix == "" {
		storageMongoCollectionPrefix = config.DefaultMongoCollectionPrefix
	}
	if storageMongoConnectTimeout <= 0 {
		storageMongoConnectTimeout = config.DefaultMongoConnectTimeoutSeconds
	}
	switch storageType {
	case config.StorageTypeMongo:
		mongoCfg := config.MongoConfig{
			URI:                   storageMongoURI,
			Database:              storageMongoDatabase,
			CollectionPrefix:      storageMongoCollectionPrefix,
			ConnectTimeoutSeconds: storageMongoConnectTimeout,
		}
		store, err = storage.NewMongoStorage(mongoCfg)
		if err != nil {
			return fmt.Errorf("failed to initialize mongo storage: %w", err)
		}
	case config.StorageTypeFile:
		store, err = storage.NewFileStorage(storagePath)
		if err != nil {
			return fmt.Errorf("failed to initialize file storage: %w", err)
		}
	default:
		store = storage.NewMemoryStorage()
	}

	// Initialize statistics collector
	statsCollector := stats.NewCollector()

	// Initialize tracing service
	tracingService := tracing.NewService(maxTraces)

	aiGenerator := ai.NewGenerator(loadAIConfig())

	// Initialize proxy engine
	scriptTimeoutMs := viper.GetInt("scripting.defaultTimeoutMs")
	proxyEngine := proxy.NewEngine(store, statsCollector, tracingService, scriptTimeoutMs)
	proxyEngine.SetAIGenerator(aiGenerator)

	// Initialize Phase 2 — GlobalStore and SessionManager
	storePath := storagePath // already resolved above
	var globalStore gvstore.GlobalStoreBackend
	if storageType == config.StorageTypeMongo {
		mongoCfg := config.MongoConfig{
			URI:                   storageMongoURI,
			Database:              storageMongoDatabase,
			CollectionPrefix:      storageMongoCollectionPrefix,
			ConnectTimeoutSeconds: storageMongoConnectTimeout,
		}
		globalStore, err = gvstore.NewMongoGlobalStoreFromConfig(mongoCfg)
		if err != nil {
			serverLog.Warn("Failed to initialize mongo global store; starting with empty store", "event", "global_store_init_failed", "error", err)
			globalStore, _ = gvstore.NewMongoGlobalStoreFromConfig(mongoCfg)
		}
	} else {
		globalStorePath := filepath.Join(storePath, "store.json")
		var fileGS *gvstore.GlobalStore
		fileGS, err = gvstore.NewGlobalStore(globalStorePath)
		if err != nil {
			serverLog.Warn("Failed to load global store; starting with empty store", "event", "global_store_init_failed", "path", globalStorePath, "error", err)
			fileGS, _ = gvstore.NewGlobalStore(globalStorePath)
		}
		globalStore = fileGS
	}

	sessionCfg := loadSessionConfig()

	sessionCtx, cancelSessions := context.WithCancel(context.Background())
	defer cancelSessions()

	var sessionManager gvstore.SessionRegistry
	switch sessionCfg.StoreType {
	case config.SessionStoreRedis:
		sessionManager, err = gvstore.NewRedisSessionManager(sessionCtx, globalStore, sessionCfg)
		if err != nil {
			return fmt.Errorf("failed to initialize redis session store: %w", err)
		}
	default:
		sessionManager = gvstore.NewSessionManager(sessionCtx, globalStore, sessionCfg)
	}
	serverLog.Info("Configured session backend",
		"event", "session_backend_configured",
		"store_type", sessionCfg.StoreType,
		"header_name", sessionCfg.HeaderName,
		"inactivity_timeout", sessionCfg.InactivityTimeout.String(),
		"max_sessions", sessionCfg.MaxSessions,
	)
	proxyEngine.SetSessionManager(sessionManager, sessionCfg.HeaderName)

	// Build the outbound HTTP client used in proxy/recording mode.
	// This replaces the default insecure-skip-verify client with one that
	// honours the proxy.* config keys (timeout, mTLS cert/key/CA).
	proxyClientCfg := proxy.ClientConfig{
		TimeoutSeconds:     viper.GetInt("proxy.timeoutSeconds"),
		InsecureSkipVerify: viper.GetBool("proxy.insecureSkipVerify"),
		CertFile:           viper.GetString("proxy.mtls.certFile"),
		KeyFile:            viper.GetString("proxy.mtls.keyFile"),
		CACertFile:         viper.GetString("proxy.mtls.caCertFile"),
	}
	if proxyClientCfg.TimeoutSeconds <= 0 {
		proxyClientCfg.TimeoutSeconds = 30
	}
	proxyHTTPClient, err := proxy.BuildClient(proxyClientCfg)
	if err != nil {
		return fmt.Errorf("failed to build proxy HTTP client: %w", err)
	}
	proxyEngine.SetProxyHTTPClient(proxyHTTPClient)

	// Resolve headless mode (flag overrides config)
	headless := viper.GetBool("server.headless")
	if headlessFlag {
		headless = true
	}

	// Build branding config
	branding := config.BrandingConfig{
		AppTitle:    viper.GetString("branding.appTitle"),
		AppSubtitle: viper.GetString("branding.appSubtitle"),
	}

	// Initialize archive manager before creating the router so all deps can
	// be passed in one shot via RouterConfig.
	var archiveManager *archive.ArchiveManager
	archivesDir := filepath.Join(storePath, "archives")
	if am, err := archive.NewArchiveManager(archivesDir, store, globalStore); err != nil {
		serverLog.Warn("Failed to initialize archive manager", "event", "archive_manager_init_failed", "path", archivesDir, "error", err)
	} else {
		archiveManager = am
	}

	// Setup router — all dependencies injected upfront, no post-construction setters.
	router := api.NewRouter(api.RouterConfig{
		Store:          store,
		StatsCollector: statsCollector,
		TracingService: tracingService,
		ProxyEngine:    proxyEngine,
		GlobalStore:    globalStore,
		SessionManager: sessionManager,
		ArchiveManager: archiveManager,
		Branding:       branding,
		Headless:       headless,
		ScriptTimeout:  scriptTimeoutMs,
		AIGenerator:    aiGenerator,
	})

	// Setup UI and docs serving (skipped in headless mode)
	if !headless {
		if devMode {
			// In dev mode, serve UI and docs from filesystem
			serverLog.Info("Serving UI from filesystem", "event", "ui_filesystem_mode", "path", "./ui/dist")
			router.ServeUIFromFS("./ui/dist")
			serverLog.Info("Serving docs from filesystem", "event", "docs_filesystem_mode", "path", "./docs")
			router.ServeDocsFromFS("./docs")
		} else {
			// In production, serve embedded UI
			uiFS, err := fs.Sub(govirtual.EmbeddedUI, "ui/dist")
			if err != nil {
				serverLog.Warn("Embedded UI not available", "event", "embedded_ui_unavailable", "error", err)
			} else {
				router.ServeEmbeddedUI(uiFS)
			}
			// Serve embedded docs
			docsFS, err := fs.Sub(govirtual.EmbeddedDocs, "docs")
			if err != nil {
				serverLog.Warn("Embedded docs not available", "event", "embedded_docs_unavailable", "error", err)
			} else {
				router.ServeEmbeddedDocs(docsFS)
			}
		}
	}

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", host, port)
	server := &http.Server{
		Handler:      router.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	var cleanup func(context.Context) error
	if tlsEnabled {
		cleanup, err = startTLSServer(server, addr, headless)
		if err != nil {
			return err
		}
	} else {
		startHTTPServer(server, addr, headless)
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	serverLog.Info("Shutting down server", "event", "server_shutdown_started")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// For TLS mode, close the mux listener first to unblock Accept() calls
	if cleanup != nil {
		if err := cleanup(ctx); err != nil {
			serverLog.Warn("Server cleanup returned an error", "event", "server_cleanup_error", "error", err)
		}
	}

	// Shutdown main server
	if err := server.Shutdown(ctx); err != nil {
		serverLog.Warn("Server shutdown returned an error", "event", "server_shutdown_error", "error", err)
	}

	serverLog.Info("Server stopped", "event", "server_shutdown_complete")
	return nil
}

func loadAIConfig() ai.Config {
	return ai.Config{
		Provider: strings.TrimSpace(viper.GetString("ai.provider")),
		OpenAI: ai.ProviderConfig{
			APIKey:  firstNonEmpty(viper.GetString("ai.openai.apiKey"), viper.GetString("ai.openaiApiKey")),
			Model:   firstNonEmpty(viper.GetString("ai.openai.model"), viper.GetString("ai.openaiModel")),
			BaseURL: firstNonEmpty(viper.GetString("ai.openai.baseUrl"), viper.GetString("ai.openaiBaseUrl")),
		},
		Claude: ai.ClaudeProviderConfig{
			APIKey:     strings.TrimSpace(viper.GetString("ai.claude.apiKey")),
			Model:      strings.TrimSpace(viper.GetString("ai.claude.model")),
			BaseURL:    strings.TrimSpace(viper.GetString("ai.claude.baseUrl")),
			APIVersion: strings.TrimSpace(viper.GetString("ai.claude.apiVersion")),
		},
	}
}

func loadSessionConfig() config.SessionConfig {
	cfg := config.SessionConfig{
		StoreType:         strings.TrimSpace(viper.GetString("session.storeType")),
		HeaderName:        viper.GetString("session.headerName"),
		InactivityTimeout: viper.GetDuration("session.inactivityTimeout"),
		MaxSessions:       viper.GetInt("session.maxSessions"),
		Redis: config.RedisSessionConfig{
			Addr:      strings.TrimSpace(viper.GetString("session.redis.addr")),
			Username:  strings.TrimSpace(viper.GetString("session.redis.username")),
			Password:  strings.TrimSpace(viper.GetString("session.redis.password")),
			DB:        viper.GetInt("session.redis.db"),
			KeyPrefix: strings.TrimSpace(viper.GetString("session.redis.keyPrefix")),
		},
	}
	cfg.Normalize()
	return cfg
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// startHTTPServer starts a plain HTTP server
func startHTTPServer(server *http.Server, addr string, headless bool) {
	server.Addr = addr
	go func() {
		logger := logging.Logger("server")
		logServerEndpoints(logger, addr, headless, false)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "event", "http_server_failed", "addr", addr, "error", err)
			os.Exit(1)
		}
	}()
}

// startTLSServer starts a server that handles both HTTP and HTTPS on the same port
// Returns a cleanup function that should be called during shutdown
func startTLSServer(server *http.Server, addr string, headless bool) (func(context.Context) error, error) {
	logger := logging.Logger("server")
	// Get TLS configuration from viper
	certFile := viper.GetString("server.tls.certFile")
	keyFile := viper.GetString("server.tls.keyFile")
	autoGenerate := viper.GetBool("server.tls.autoGenerate")
	tlsStorePath := viper.GetString("server.tls.storePath")
	storagePath := viper.GetString("storage.path")

	// Resolve TLS store path - default to <storage.path>/certs if not configured
	if tlsStorePath == "" {
		// Resolve relative storage path
		if storagePath != "" && !filepath.IsAbs(storagePath) {
			cwd, _ := os.Getwd()
			storagePath = filepath.Join(cwd, storagePath)
		}
		tlsStorePath = filepath.Join(storagePath, "certs")
	}

	// Get or generate TLS certificate
	certManager := tlsutil.NewCertificateManager(certFile, keyFile, tlsStorePath)

	cert, err := certManager.GetCertificate(autoGenerate)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS certificate: %w", err)
	}

	certPath, keyPath := certManager.GetCertificatePaths()
	logger.Info("Using TLS certificate", "event", "tls_certificate_loaded", "cert_path", certPath, "key_path", keyPath)

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Create base listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// Create multiplexed listener for HTTP and HTTPS on same port
	muxListener := tlsutil.NewMuxListener(listener, tlsConfig)

	// Start HTTP server (for redirects or plain HTTP if needed)
	httpServer := &http.Server{
		Handler:      server.Handler,
		ReadTimeout:  server.ReadTimeout,
		WriteTimeout: server.WriteTimeout,
		IdleTimeout:  server.IdleTimeout,
	}

	go func() {
		logServerEndpoints(logger, addr, headless, true)

		// Serve HTTPS
		go func() {
			if err := server.Serve(muxListener.HTTPSListener()); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTPS server error", "event", "https_server_error", "addr", addr, "error", err)
			}
		}()

		// Serve HTTP
		if err := httpServer.Serve(muxListener.HTTPListener()); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "event", "http_server_error", "addr", addr, "error", err)
		}
	}()

	// Return cleanup function
	return func(ctx context.Context) error {
		// Close the mux listener first to unblock Accept() calls
		muxListener.Close()
		// Shutdown the HTTP server (HTTPS server is shut down by main)
		return httpServer.Shutdown(ctx)
	}, nil
}

func logServerEndpoints(logger interface {
	Info(string, ...any)
}, addr string, headless, tls bool) {
	if tls {
		logger.Info("Starting Go-Virtual server", "event", "server_start", "addr", addr, "tls", true)
	} else {
		logger.Info("Starting Go-Virtual server", "event", "server_start", "addr", addr, "tls", false)
	}
	if headless {
		logger.Info("Headless mode enabled", "event", "headless_mode_enabled", "addr", addr)
		return
	}
	if tls {
		logger.Info("Admin UI available", "event", "admin_ui_available", "https_url", fmt.Sprintf("https://%s/_ui/", addr), "http_url", fmt.Sprintf("http://%s/_ui/", addr))
		logger.Info("Admin API available", "event", "admin_api_available", "https_url", fmt.Sprintf("https://%s/_api/", addr), "http_url", fmt.Sprintf("http://%s/_api/", addr))
		logger.Info("Documentation available", "event", "docs_available", "https_url", fmt.Sprintf("https://%s/_docs/", addr), "http_url", fmt.Sprintf("http://%s/_docs/", addr))
		return
	}
	logger.Info("Admin UI available", "event", "admin_ui_available", "url", fmt.Sprintf("http://%s/_ui/", addr))
	logger.Info("Admin API available", "event", "admin_api_available", "url", fmt.Sprintf("http://%s/_api/", addr))
	logger.Info("Documentation available", "event", "docs_available", "url", fmt.Sprintf("http://%s/_docs/", addr))
}
