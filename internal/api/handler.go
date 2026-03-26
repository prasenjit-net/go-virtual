package api

import (
	"strings"
	"time"

	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/parser"
	"github.com/prasenjit/go-virtual/internal/proxy"
	"github.com/prasenjit/go-virtual/internal/scripting"
	"github.com/prasenjit/go-virtual/internal/stats"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
	"github.com/prasenjit/go-virtual/internal/template"
	"github.com/prasenjit/go-virtual/internal/tracing"
)

// HandlerConfig holds all dependencies for the API handler.
type HandlerConfig struct {
	Store          storage.Storage
	StatsCollector *stats.Collector
	TracingService *tracing.Service
	ProxyEngine    *proxy.Engine
	GlobalStore    *store.GlobalStore      // optional; nil = Phase 1 mode
	SessionManager *store.SessionManager   // optional; nil = Phase 1 mode
	ArchiveManager *archive.ArchiveManager // optional; nil disables archive endpoints
	Branding       config.BrandingConfig
	ScriptTimeout  int // ms; 0 = use default (100)
}

// Handler handles API requests
type Handler struct {
	store          storage.Storage
	statsCollector *stats.Collector
	tracingService *tracing.Service
	proxyEngine    *proxy.Engine
	parser         *parser.Parser
	templateEngine *template.Engine
	scriptEngine   *scripting.ScriptEngine
	globalStore    *store.GlobalStore
	sessionManager *store.SessionManager
	archiveManager *archive.ArchiveManager
	branding       config.BrandingConfig
}

// NewHandler creates a fully-initialised Handler from a HandlerConfig.
func NewHandler(cfg HandlerConfig) *Handler {
	timeout := cfg.ScriptTimeout
	if timeout <= 0 {
		timeout = 100
	}
	b := cfg.Branding
	if b.AppTitle == "" {
		b.AppTitle = "go-virtual"
	}
	if b.AppSubtitle == "" {
		b.AppSubtitle = "API Mock & Virtualization"
	}
	h := &Handler{
		store:          cfg.Store,
		statsCollector: cfg.StatsCollector,
		tracingService: cfg.TracingService,
		proxyEngine:    cfg.ProxyEngine,
		globalStore:    cfg.GlobalStore,
		sessionManager: cfg.SessionManager,
		archiveManager: cfg.ArchiveManager,
		branding:       b,
		parser:         parser.NewParser(),
		templateEngine: template.NewEngine(),
		scriptEngine:   scripting.NewScriptEngine(cfg.Store, timeout),
	}
	if cfg.GlobalStore != nil {
		h.scriptEngine.SetGlobalStore(cfg.GlobalStore)
	}
	return h
}

// ── Shared helpers ───────────────────────────────────────────────────────────

// generateID generates a unique ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of n chars
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// normalizeTag returns a lower-cased, trimmed tag name.
func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}
