package api

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/ai"
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
	GlobalStore    store.GlobalStoreBackend // optional; nil = Phase 1 mode
	SessionManager store.SessionRegistry    // optional; nil = Phase 1 mode
	ArchiveManager archive.ArchiveService   // optional; nil disables archive endpoints
	Branding       config.BrandingConfig
	ScriptTimeout  int           // ms; 0 = use default (100)
	AIGenerator    *ai.Generator // optional; nil = AI generation disabled
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
	globalStore    store.GlobalStoreBackend
	sessionManager store.SessionRegistry
	archiveManager archive.ArchiveService
	branding       config.BrandingConfig
	aiGenerator    *ai.Generator
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
		aiGenerator:    cfg.AIGenerator,
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
	return time.Now().UTC().Format("20060102150405") + "-" + uuid.NewString()[:8]
}

// normalizeTag returns a lower-cased, trimmed tag name.
func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}
