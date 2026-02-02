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
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
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
	devMode   bool
	portFlag  int
	tlsFlag   bool
)

func init() {
	serveCmd.Flags().BoolVar(&devMode, "dev", false, "Enable development mode (serve UI from filesystem)")
	serveCmd.Flags().IntVarP(&portFlag, "port", "p", 0, "Override server port")
	serveCmd.Flags().BoolVar(&tlsFlag, "tls", false, "Enable TLS (overrides config)")

	// Bind flags to viper
	viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("server.tls.enabled", serveCmd.Flags().Lookup("tls"))
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
	proxyEngine := proxy.NewEngine(store, statsCollector, tracingService)

	// Setup router
	router := api.NewRouter(store, statsCollector, tracingService, proxyEngine)

	// Setup UI serving
	if devMode {
		// In dev mode, serve UI from filesystem
		log.Println("Development mode: Serving UI from ./ui/dist")
		router.ServeUIFromFS("./ui/dist")
	} else {
		// In production, serve embedded UI
		uiFS, err := fs.Sub(govirtual.EmbeddedUI, "ui/dist")
		if err != nil {
			log.Printf("Warning: Embedded UI not available: %v", err)
		} else {
			router.ServeEmbeddedUI(uiFS)
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
		cleanup = startTLSServer(server, addr)
	} else {
		startHTTPServer(server, addr)
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
func startHTTPServer(server *http.Server, addr string) {
	server.Addr = addr
	go func() {
		log.Printf("Starting Go-Virtual server on %s", addr)
		log.Printf("Admin UI available at http://%s/_ui/", addr)
		log.Printf("Admin API available at http://%s/_api/", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()
}

// startTLSServer starts a server that handles both HTTP and HTTPS on the same port
// Returns a cleanup function that should be called during shutdown
func startTLSServer(server *http.Server, addr string) func(context.Context) error {
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
		log.Printf("Admin UI available at https://%s/_ui/ (or http://%s/_ui/)", addr, addr)
		log.Printf("Admin API available at https://%s/_api/ (or http://%s/_api/)", addr, addr)

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
