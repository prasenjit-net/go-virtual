package logging

import (
	"bytes"
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/config"
)

func TestNormalizeConfigWarnings(t *testing.T) {
	cfg, warnings := normalizeConfig(config.LoggingConfig{Level: "verbose", Format: "pretty"})
	if cfg.Level != config.LogLevelInfo {
		t.Fatalf("expected info fallback, got %q", cfg.Level)
	}
	if cfg.Format != config.LogFormatJSON {
		t.Fatalf("expected json fallback, got %q", cfg.Format)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected warnings for invalid level and format, got %v", warnings)
	}
}

func TestAccessMiddlewareHealthUsesDebugLevel(t *testing.T) {
	handler := &recordingHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessMiddleware())
	r.GET("/_api/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/_api/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := handler.last(); got.Level != slog.LevelDebug {
		t.Fatalf("expected debug level access log, got %v", got.Level)
	}
	if got := handler.last().Attrs["event"]; got != "http_request" {
		t.Fatalf("expected http_request event, got %v", got)
	}
}

func TestRecoveryMiddlewareLogsPanic(t *testing.T) {
	handler := &recordingHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RecoveryMiddleware())
	r.GET("/boom", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from recovery middleware, got %d", w.Code)
	}
	if got := handler.last(); got.Level != slog.LevelError {
		t.Fatalf("expected error level panic log, got %v", got.Level)
	}
	if got := handler.last().Attrs["event"]; got != "http_panic" {
		t.Fatalf("expected http_panic event, got %v", got)
	}
}

func TestSimpleTextHandlerFormatsReadableLine(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewSimpleTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	logger.With("component", "server").Info("Starting server",
		"event", "server_start",
		"addr", "127.0.0.1:8080",
		"tls", false,
	)

	line := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(line, "INFO server: Starting server") {
		t.Fatalf("expected simple text prefix, got %q", line)
	}
	if !strings.Contains(line, "addr=127.0.0.1:8080") || !strings.Contains(line, "tls=false") {
		t.Fatalf("expected readable attrs in text log, got %q", line)
	}
	if strings.Contains(line, "event=") || strings.Contains(line, "time=") {
		t.Fatalf("expected simple text log without event/time noise, got %q", line)
	}
}

func TestSetupConfiguresGlobalLoggingBridges(t *testing.T) {
	origDefault := slog.Default()
	origGinWriter := gin.DefaultWriter
	origGinErrorWriter := gin.DefaultErrorWriter
	origLogWriter := log.Writer()
	origFlags := log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(origDefault)
		gin.DefaultWriter = origGinWriter
		gin.DefaultErrorWriter = origGinErrorWriter
		log.SetOutput(origLogWriter)
		log.SetFlags(origFlags)
	})

	logger, warnings := Setup(config.LoggingConfig{Level: config.LogLevelDebug, Format: config.LogFormatText})
	if logger == nil {
		t.Fatal("expected logger")
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if slog.Default() != logger {
		t.Fatal("expected setup to install default logger")
	}
	if _, ok := gin.DefaultWriter.(*LevelWriter); !ok {
		t.Fatalf("expected gin default writer to be LevelWriter, got %T", gin.DefaultWriter)
	}
	if _, ok := gin.DefaultErrorWriter.(*LevelWriter); !ok {
		t.Fatalf("expected gin default error writer to be LevelWriter, got %T", gin.DefaultErrorWriter)
	}
	if log.Flags() != 0 {
		t.Fatalf("expected stdlib log flags to be cleared, got %d", log.Flags())
	}
}

func TestLoggerUsesComponentWhenProvided(t *testing.T) {
	handler := &recordingHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	Logger("worker").Info("job completed")
	if got := handler.last().Attrs["component"]; got != "worker" {
		t.Fatalf("expected component attr, got %v", got)
	}

	Logger("").Info("root logger")
	if _, ok := handler.last().Attrs["component"]; ok {
		t.Fatalf("expected empty component logger to not add component attr, got %v", handler.last().Attrs)
	}
}

func TestLevelWriterBuffersUntilNewlineAndFlushes(t *testing.T) {
	handler := &recordingHandler{}
	handler.ensureEntries()
	writer := &LevelWriter{
		logger: slog.New(handler),
		level:  slog.LevelWarn,
	}

	if _, err := writer.Write([]byte("partial")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := len(*handler.entries); got != 0 {
		t.Fatalf("expected no log entry before newline, got %d", got)
	}

	if _, err := writer.Write([]byte(" line\nnext line")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := len(*handler.entries); got != 1 {
		t.Fatalf("expected one log entry after newline, got %d", got)
	}
	if got := (*handler.entries)[0].Message; got != "partial line" {
		t.Fatalf("expected merged line, got %q", got)
	}
	if got := (*handler.entries)[0].Level; got != slog.LevelWarn {
		t.Fatalf("expected warn level, got %v", got)
	}

	writer.Flush()
	if got := len(*handler.entries); got != 2 {
		t.Fatalf("expected second log entry after flush, got %d", got)
	}
	if got := (*handler.entries)[1].Message; got != "next line" {
		t.Fatalf("expected flushed line, got %q", got)
	}
}

func TestSimpleTextHandlerFormatsGroupsAndTypedValues(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewSimpleTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})).
		WithGroup("req").
		With("component", "proxy")

	when := time.Date(2026, 4, 25, 23, 0, 0, 0, time.UTC)
	logger.Info("served",
		"path", "/users 1",
		"duration", 2*time.Second,
		"count", 2,
		"ok", true,
		"when", when,
		"err", errors.New("boom"),
		"list", []string{"a", "b"},
	)

	line := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(line, "INFO: served") {
		t.Fatalf("expected simple text prefix, got %q", line)
	}
	for _, want := range []string{
		`req.component=proxy`,
		`req.path="/users 1"`,
		`req.duration=2s`,
		`req.count=2`,
		`req.ok=true`,
		`req.when=2026-04-25T23:00:00Z`,
		`req.err="boom"`,
		`req.list="[a b]"`,
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected %q in line %q", want, line)
		}
	}
}

func TestAccessHelpers(t *testing.T) {
	if got := routeOrPath("/pets/{id}", "/pets/1"); got != "/pets/{id}" {
		t.Fatalf("expected route to win, got %q", got)
	}
	if got := routeOrPath("", "/pets/1"); got != "/pets/1" {
		t.Fatalf("expected request path fallback, got %q", got)
	}

	cases := []struct {
		path   string
		area   string
		status int
		level  slog.Level
	}{
		{path: "/_api/health", area: "admin-api", status: http.StatusOK, level: slog.LevelDebug},
		{path: "/_prometheus", area: "metrics", status: http.StatusOK, level: slog.LevelDebug},
		{path: "/_ui/app.js", area: "ui-asset", status: http.StatusOK, level: slog.LevelDebug},
		{path: "/_docs/openapi.json", area: "docs-asset", status: http.StatusOK, level: slog.LevelDebug},
		{path: "/pets", area: "proxy", status: http.StatusBadRequest, level: slog.LevelWarn},
		{path: "/pets", area: "proxy", status: http.StatusInternalServerError, level: slog.LevelError},
		{path: "/pets", area: "proxy", status: http.StatusOK, level: slog.LevelInfo},
	}
	for _, tc := range cases {
		if got := accessLevel(tc.area, tc.path, tc.status); got != tc.level {
			t.Fatalf("accessLevel(%q, %q, %d) = %v, want %v", tc.area, tc.path, tc.status, got, tc.level)
		}
	}

	areas := map[string]string{
		"/_prometheus":     "metrics",
		"/_api/specs":      "admin-api",
		"/_ui/index.html":  "ui-asset",
		"/_ui/dashboard":   "ui",
		"/_docs/app.js":    "docs-asset",
		"/_docs/reference": "docs",
		"/pets":            "proxy",
	}
	for path, want := range areas {
		if got := classifyArea(path); got != want {
			t.Fatalf("classifyArea(%q) = %q, want %q", path, got, want)
		}
	}

	if !hasStaticExtension("/assets/app.css") {
		t.Fatal("expected css extension to be static")
	}
	if hasStaticExtension("/_ui/dashboard") {
		t.Fatal("expected route without extension to not be static")
	}
}

func TestLevelAndQuoteHelpers(t *testing.T) {
	if got := levelFromString(config.LogLevelDebug); got != slog.LevelDebug {
		t.Fatalf("expected debug level, got %v", got)
	}
	if got := levelFromString(config.LogLevelWarn); got != slog.LevelWarn {
		t.Fatalf("expected warn level, got %v", got)
	}
	if got := levelFromString(config.LogLevelError); got != slog.LevelError {
		t.Fatalf("expected error level, got %v", got)
	}
	if got := levelFromString("unknown"); got != slog.LevelInfo {
		t.Fatalf("expected info fallback, got %v", got)
	}

	if got := quote("  "); got != `""` {
		t.Fatalf("expected empty quote, got %q", got)
	}
	if got := quote("  hello "); got != `"hello"` {
		t.Fatalf("expected trimmed quote, got %q", got)
	}
}

type recordedEntry struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

type recordingHandler struct {
	entries *[]recordedEntry
	attrs   []slog.Attr
	group   string
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, record slog.Record) error {
	h.ensureEntries()
	entry := recordedEntry{
		Level:   record.Level,
		Message: record.Message,
		Attrs:   map[string]any{},
	}
	for _, attr := range h.attrs {
		entry.Attrs[h.attrKey(attr.Key)] = attr.Value.Any()
	}
	record.Attrs(func(attr slog.Attr) bool {
		entry.Attrs[h.attrKey(attr.Key)] = attr.Value.Any()
		return true
	})
	*h.entries = append(*h.entries, entry)
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.ensureEntries()
	cloned := *h
	cloned.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cloned
}

func (h *recordingHandler) WithGroup(name string) slog.Handler {
	h.ensureEntries()
	cloned := *h
	if h.group == "" {
		cloned.group = name
	} else {
		cloned.group = h.group + "." + name
	}
	return &cloned
}

func (h *recordingHandler) last() recordedEntry {
	h.ensureEntries()
	if len(*h.entries) == 0 {
		return recordedEntry{}
	}
	return (*h.entries)[len(*h.entries)-1]
}

func (h *recordingHandler) attrKey(key string) string {
	if h.group == "" {
		return key
	}
	return h.group + "." + key
}

func (h *recordingHandler) ensureEntries() {
	if h.entries == nil {
		h.entries = &[]recordedEntry{}
	}
}
