package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	govirtual "github.com/prasenjit/go-virtual"
	"github.com/prasenjit/go-virtual/internal/api"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/tlsutil"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	devMode := flag.Bool("dev", false, "Enable development mode (serve UI from filesystem)")
	port := flag.Int("port", 0, "Override server port")
	enableTLS := flag.Bool("tls", false, "Enable TLS (overrides config)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: Could not load config file: %v. Using defaults.", err)
		cfg = config.Default()
	}

	// Override port if specified
	if *port > 0 {
		cfg.Server.Port = *port
	}

	// Override TLS if specified via flag
	if *enableTLS {
		cfg.Server.TLS.Enabled = true
	}

	// Initialize storage
	var store storage.Storage
	if cfg.Storage.Type == "file" {
		store, err = storage.NewFileStorage(cfg.Storage.Path)
		if err != nil {
			log.Fatalf("Failed to initialize file storage: %v", err)
		}
	} else {
		store = storage.NewMemoryStorage()
	}

	// Initialize statistics collector
	statsCollector := stats.NewCollector()

	// Initialize tracing service
	tracingService := tracing.NewService(cfg.Tracing.MaxTraces)

	// Initialize proxy engine
	proxyEngine := proxy.NewEngine(store, statsCollector, tracingService)

	// Setup router
	router := api.NewRouter(store, statsCollector, tracingService, proxyEngine)

	// Setup UI serving
	if *devMode {
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
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Handler:      router.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	if cfg.Server.TLS.Enabled {
		startTLSServer(server, addr, cfg)
	} else {
		startHTTPServer(server, addr)
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
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
func startTLSServer(server *http.Server, addr string, cfg *config.Config) {
	// Get or generate TLS certificate
	certManager := tlsutil.NewCertificateManager(
		cfg.Server.TLS.CertFile,
		cfg.Server.TLS.KeyFile,
		cfg.Server.TLS.StorePath,
	)

	cert, err := certManager.GetCertificate(cfg.Server.TLS.AutoGenerate)
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
}
