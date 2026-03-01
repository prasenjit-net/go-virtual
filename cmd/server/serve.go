package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	govirtual "github.com/prasenjit/go-virtual"
	"github.com/prasenjit/go-virtual/internal/api"
	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/config"
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
	devMode     bool
	portFlag    int
	tlsFlag     bool
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
	log.Printf("Using data directory: %s", storagePath)

	// Initialize storage
	var store storage.Storage
	var err error
	if storageType == "file" {
		store, err = storage.NewFileStorage(storagePath)
		if err != nil {
			return fmt.Errorf("failed to initialize file storage: %w", err)
		}
	} else {
		store = storage.NewMemoryStorage()
	}

	// Initialize statistics collector
	statsCollector := stats.NewCollector()

	// Initialize tracing service
	tracingService := tracing.NewService(maxTraces)

	// Initialize proxy engine
	scriptTimeoutMs := viper.GetInt("scripting.defaultTimeoutMs")
	proxyEngine := proxy.NewEngine(store, statsCollector, tracingService, scriptTimeoutMs)

	// Initialize Phase 2 — GlobalStore and SessionManager
	storePath := storagePath // already resolved above
	globalStorePath := filepath.Join(storePath, "store.json")

	globalStore, err := gvstore.NewGlobalStore(globalStorePath)
	if err != nil {
		log.Printf("Warning: failed to load global store from %s: %v — starting with empty store", globalStorePath, err)
		globalStore, _ = gvstore.NewGlobalStore(globalStorePath)
	}

	sessionCfg := config.SessionConfig{
		HeaderName:        viper.GetString("session.headerName"),
		InactivityTimeout: viper.GetDuration("session.inactivityTimeout"),
		MaxSessions:       viper.GetInt("session.maxSessions"),
	}
	if sessionCfg.HeaderName == "" {
		sessionCfg.HeaderName = "X-Virtual-Session-Id"
	}
	if sessionCfg.InactivityTimeout <= 0 {
		sessionCfg.InactivityTimeout = 30 * time.Minute
	}
	if sessionCfg.MaxSessions <= 0 {
		sessionCfg.MaxSessions = 10000
	}

	sessionCtx, cancelSessions := context.WithCancel(context.Background())
	defer cancelSessions()

	sessionManager := gvstore.NewSessionManager(sessionCtx, globalStore, sessionCfg)
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
		log.Printf("Warning: failed to init archive manager at %s: %v", archivesDir, err)
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
	})

	// Setup UI and docs serving (skipped in headless mode)
	if !headless {
		if devMode {
			// In dev mode, serve UI and docs from filesystem
			log.Println("Development mode: Serving UI from ./ui/dist")
			router.ServeUIFromFS("./ui/dist")
			log.Println("Development mode: Serving docs from ./docs")
			router.ServeDocsFromFS("./docs")
		} else {
			// In production, serve embedded UI
			uiFS, err := fs.Sub(govirtual.EmbeddedUI, "ui/dist")
			if err != nil {
				log.Printf("Warning: Embedded UI not available: %v", err)
			} else {
				router.ServeEmbeddedUI(uiFS)
			}
			// Serve embedded docs
			docsFS, err := fs.Sub(govirtual.EmbeddedDocs, "docs")
			if err != nil {
				log.Printf("Warning: Embedded docs not available: %v", err)
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
		cleanup = startTLSServer(server, addr, headless)
	} else {
		startHTTPServer(server, addr, headless)
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// For TLS mode, close the mux listener first to unblock Accept() calls
	if cleanup != nil {
		if err := cleanup(ctx); err != nil {
			log.Printf("Cleanup error: %v", err)
		}
	}

	// Shutdown main server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
	return nil
}

// startHTTPServer starts a plain HTTP server
func startHTTPServer(server *http.Server, addr string, headless bool) {
	server.Addr = addr
	go func() {
		log.Printf("Starting Go-Virtual server on %s", addr)
		if headless {
			log.Printf("Headless mode: admin API and UI disabled, serving proxy only")
		} else {
			log.Printf("Admin UI available at http://%s/_ui/", addr)
			log.Printf("Admin API available at http://%s/_api/", addr)
			log.Printf("Documentation available at http://%s/_docs/", addr)
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()
}

// startTLSServer starts a server that handles both HTTP and HTTPS on the same port
// Returns a cleanup function that should be called during shutdown
func startTLSServer(server *http.Server, addr string, headless bool) func(context.Context) error {
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
		log.Fatalf("Failed to get TLS certificate: %v", err)
	}

	certPath, keyPath := certManager.GetCertificatePaths()
	log.Printf("Using TLS certificate: %s", certPath)
	log.Printf("Using TLS private key: %s", keyPath)

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Create base listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
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
		log.Printf("Starting Go-Virtual server on %s (HTTP & HTTPS)", addr)
		if headless {
			log.Printf("Headless mode: admin API and UI disabled, serving proxy only")
		} else {
			log.Printf("Admin UI available at https://%s/_ui/ (or http://%s/_ui/)", addr, addr)
			log.Printf("Admin API available at https://%s/_api/ (or http://%s/_api/)", addr, addr)
			log.Printf("Documentation available at https://%s/_docs/ (or http://%s/_docs/)", addr, addr)
		}

		// Serve HTTPS
		go func() {
			if err := server.Serve(muxListener.HTTPSListener()); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTPS server error: %v", err)
			}
		}()

		// Serve HTTP
		if err := httpServer.Serve(muxListener.HTTPListener()); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Return cleanup function
	return func(ctx context.Context) error {
		// Close the mux listener first to unblock Accept() calls
		muxListener.Close()
		// Shutdown the HTTP server (HTTPS server is shut down by main)
		return httpServer.Shutdown(ctx)
	}
}
