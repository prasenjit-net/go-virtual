package api

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

// Router handles HTTP routing
type Router struct {
	engine         *gin.Engine
	store          storage.Storage
	statsCollector *stats.Collector
	tracingService *tracing.Service
	proxyEngine    *proxy.Engine
	handler        *Handler
	headless       bool
	branding       config.BrandingConfig
}

// NewRouter creates a new router
func NewRouter(store storage.Storage, statsCollector *stats.Collector, tracingService *tracing.Service, proxyEngine *proxy.Engine, headless bool, branding ...config.BrandingConfig) *Router {
	gin.SetMode(gin.ReleaseMode)

	var b config.BrandingConfig
	if len(branding) > 0 {
		b = branding[0]
	}

	r := &Router{
		engine:         gin.New(),
		store:          store,
		statsCollector: statsCollector,
		tracingService: tracingService,
		proxyEngine:    proxyEngine,
		headless:       headless,
		branding:       b,
	}

	// Create handler
	r.handler = NewHandler(store, statsCollector, tracingService, proxyEngine)
	r.handler.SetBranding(b)

	// Setup middleware
	r.engine.Use(gin.Recovery())
	r.engine.Use(corsMiddleware())
	r.engine.Use(gin.Logger())

	// Setup routes
	r.setupRoutes()

	return r
}

// setupRoutes configures all routes
func (r *Router) setupRoutes() {
	// Prometheus metrics endpoint — always available, even in headless mode.
	r.engine.GET("/_prometheus", gin.WrapH(promhttp.Handler()))

	if r.headless {
		// Headless mode: no admin API, no UI – proxy only
		r.engine.NoRoute(func(c *gin.Context) {
			r.proxyEngine.ServeHTTP(c.Writer, c.Request)
		})
		return
	}

	// Admin API routes
	api := r.engine.Group("/_api")
	{
		// Specs
		api.GET("/specs", r.handler.ListSpecs)
		api.POST("/specs", r.handler.CreateSpec)
		api.GET("/specs/:id", r.handler.GetSpec)
		api.PUT("/specs/:id", r.handler.UpdateSpec)
		api.DELETE("/specs/:id", r.handler.DeleteSpec)
		api.PUT("/specs/:id/enable", r.handler.EnableSpec)
		api.PUT("/specs/:id/disable", r.handler.DisableSpec)
		api.PUT("/specs/:id/tracing", r.handler.ToggleTracing)
		api.PUT("/specs/:id/example-fallback", r.handler.ToggleExampleFallback)
		api.PUT("/specs/:id/backend", r.handler.SetBackendURI)
		api.PUT("/specs/:id/proxy-mode", r.handler.ToggleProxyMode)
		api.GET("/specs/:id/tags", r.handler.GetSpecTags)
		api.PUT("/specs/:id/tags", r.handler.UpdateSpecTags)

		// Operations
		api.GET("/specs/:id/operations", r.handler.ListOperations)
		api.GET("/operations/:id", r.handler.GetOperation)
		api.GET("/operations/:id/signature", r.handler.GetSignatureConfig)
		api.PUT("/operations/:id/signature", r.handler.UpdateSignatureConfig)

		// Response Configs
		api.GET("/operations/:id/responses", r.handler.ListResponseConfigs)
		api.POST("/operations/:id/responses", r.handler.CreateResponseConfig)
		api.GET("/responses/:id", r.handler.GetResponseConfig)
		api.PUT("/responses/:id", r.handler.UpdateResponseConfig)
		api.DELETE("/responses/:id", r.handler.DeleteResponseConfig)
		api.PUT("/responses/:id/priority", r.handler.UpdateResponsePriority)

		// Script bindings (per operation)
		api.GET("/operations/:id/scripts", r.handler.ListScriptBindings)
		api.POST("/operations/:id/scripts", r.handler.CreateScriptBinding)
		api.PUT("/operations/:id/scripts/reorder", r.handler.ReorderScriptBindings)
		api.PUT("/operations/:id/scripts/:bindingId", r.handler.UpdateScriptBinding)
		api.DELETE("/operations/:id/scripts/:bindingId", r.handler.DeleteScriptBinding)

		// Tags
		api.GET("/tags", r.handler.ListTags)
		api.POST("/tags", r.handler.CreateTag)
		api.PUT("/tags/:name", r.handler.UpdateTag)
		api.DELETE("/tags/:name", r.handler.DeleteTag)

		// Statistics
		api.GET("/stats", r.handler.GetGlobalStats)
		api.GET("/stats/specs/:id", r.handler.GetSpecStats)
		api.GET("/stats/operations/:id", r.handler.GetOperationStats)
		api.POST("/stats/reset", r.handler.ResetStats)

		// Tracing
		api.GET("/traces", r.handler.ListTraces)
		api.GET("/traces/:id", r.handler.GetTrace)
		api.DELETE("/traces", r.handler.ClearTraces)

		// Routes info
		api.GET("/routes", r.handler.GetRoutes)

		// Health
		api.GET("/health", r.handler.HealthCheck)
		api.GET("/version", r.handler.Version)
		api.GET("/branding", r.handler.GetBranding)

		// Template validation
		api.POST("/templates/validate", r.handler.ValidateTemplate)

		// Scripts
		api.GET("/scripts", r.handler.ListScripts)
		api.POST("/scripts", r.handler.CreateScript)
		api.POST("/scripts/validate", r.handler.ValidateScript)
		api.GET("/scripts/:id", r.handler.GetScript)
		api.PUT("/scripts/:id", r.handler.UpdateScript)
		api.DELETE("/scripts/:id", r.handler.DeleteScript)
		api.POST("/scripts/:id/test", r.handler.TestScript)

		// Global store (Phase 2)
		api.GET("/store", r.handler.ListStoreEntries)
		api.GET("/store/:key", r.handler.GetStoreEntry)
		api.PUT("/store/:key", r.handler.UpsertStoreEntry)
		api.DELETE("/store/:key", r.handler.DeleteStoreEntry)
		api.DELETE("/store", r.handler.ClearStore)

		// Sessions (Phase 2)
		api.GET("/sessions", r.handler.ListSessions)
		api.GET("/sessions/:id", r.handler.GetSession)
		api.DELETE("/sessions/:id", r.handler.InvalidateSession)
		api.DELETE("/sessions", r.handler.InvalidateAllSessions)

		// Archives
		api.GET("/archives", r.handler.ListArchives)
		api.POST("/archives", r.handler.CreateArchive)
		api.POST("/archives/upload", r.handler.UploadArchive)
		api.GET("/archives/:id", r.handler.GetArchive)
		api.DELETE("/archives/:id", r.handler.DeleteArchive)
		api.GET("/archives/:id/download", r.handler.DownloadArchive)
		api.POST("/archives/:id/restore", r.handler.RestoreArchive)
	}

	// WebSocket for live tracing
	wsHandler := tracing.NewWebSocketHandler(r.tracingService)
	r.engine.GET("/_api/traces/stream", gin.WrapH(wsHandler))
}

// ServeUIFromFS serves the UI from the filesystem (for development)
func (r *Router) ServeUIFromFS(dir string) {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Directory doesn't exist, create a placeholder handler
		r.engine.GET("/_ui/*filepath", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "UI not built",
				"message": "Run 'make build-ui' or 'npm run build' in the ui directory",
			})
		})
		return
	}

	// Serve static files
	r.engine.Static("/_ui", dir)

	// Handle SPA routing - serve index.html for non-file requests
	r.engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// If it's a UI route, serve index.html
		if strings.HasPrefix(path, "/_ui") {
			indexPath := filepath.Join(dir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				c.File(indexPath)
				return
			}
		}

		// Otherwise, try the proxy
		r.proxyEngine.ServeHTTP(c.Writer, c.Request)
	})
}

// ServeEmbeddedUI serves the UI from embedded files (for production)
func (r *Router) ServeEmbeddedUI(uiFS fs.FS) {
	// Serve embedded static files
	staticServer := http.FileServer(http.FS(uiFS))
	
	r.engine.GET("/_ui/*filepath", func(c *gin.Context) {
		// Remove /_ui prefix for file serving
		path := strings.TrimPrefix(c.Param("filepath"), "/")
		
		// Check if file exists
		if f, err := uiFS.Open(path); err == nil {
			f.Close()
			http.StripPrefix("/_ui", staticServer).ServeHTTP(c.Writer, c.Request)
			return
		}

		// For SPA, serve index.html for non-existent paths
		if f, err := uiFS.Open("index.html"); err == nil {
			defer f.Close()
			c.Header("Content-Type", "text/html")
			data, _ := fs.ReadFile(uiFS, "index.html")
			c.Data(http.StatusOK, "text/html", data)
			return
		}

		c.Status(http.StatusNotFound)
	})

	// Handle redirect from /_ui to /_ui/
	r.engine.GET("/_ui", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/_ui/")
	})

	// Setup NoRoute handler for proxy
	r.engine.NoRoute(func(c *gin.Context) {
		r.proxyEngine.ServeHTTP(c.Writer, c.Request)
	})
}

// ServeDocsFromFS serves the documentation from the filesystem (for development)
func (r *Router) ServeDocsFromFS(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		r.engine.GET("/_docs/*filepath", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "docs not found"})
		})
		return
	}

	r.engine.Static("/_docs", dir)

	r.engine.GET("/_docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/_docs/")
	})
}

// ServeEmbeddedDocs serves the documentation from embedded files (for production)
func (r *Router) ServeEmbeddedDocs(docsFS fs.FS) {
	staticServer := http.FileServer(http.FS(docsFS))

	r.engine.GET("/_docs/*filepath", func(c *gin.Context) {
		path := strings.TrimPrefix(c.Param("filepath"), "/")
		if path == "" {
			path = "index.html"
		}

		if f, err := docsFS.Open(path); err == nil {
			f.Close()
			http.StripPrefix("/_docs", staticServer).ServeHTTP(c.Writer, c.Request)
			return
		}

		c.Status(http.StatusNotFound)
	})

	r.engine.GET("/_docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/_docs/")
	})
}

// SetStoreManager wires Phase 2 GlobalStore and SessionManager into the handler.
func (r *Router) SetStoreManager(gs *store.GlobalStore, sm *store.SessionManager) {
	r.handler.SetStoreManager(gs, sm)
}

// SetArchiveManager wires the ArchiveManager into the handler.
func (r *Router) SetArchiveManager(am *archive.ArchiveManager) {
	r.handler.SetArchiveManager(am)
}

// Handler returns the http.Handler
func (r *Router) Handler() http.Handler {
	return r.engine
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
