package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/config"
)

// Setup configures the shared application logger and installs bridges for
// standard-library logging and Gin's default writers.
func Setup(cfg config.LoggingConfig) (*slog.Logger, []string) {
	normalized, warnings := normalizeConfig(cfg)

	opts := &slog.HandlerOptions{Level: levelFromString(normalized.Level)}
	var handler slog.Handler
	switch normalized.Format {
	case config.LogFormatText:
		handler = NewSimpleTextHandler(os.Stderr, opts)
	default:
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	log.SetFlags(0)
	log.SetOutput(&LevelWriter{logger: logger.With("component", "stdlib"), level: slog.LevelInfo})
	gin.DefaultWriter = &LevelWriter{logger: logger.With("component", "gin"), level: slog.LevelInfo}
	gin.DefaultErrorWriter = &LevelWriter{logger: logger.With("component", "gin"), level: slog.LevelError}

	return logger, warnings
}

// Logger returns a component-scoped logger derived from the shared default logger.
func Logger(component string) *slog.Logger {
	if component == "" {
		return slog.Default()
	}
	return slog.Default().With("component", component)
}

func normalizeConfig(cfg config.LoggingConfig) (config.LoggingConfig, []string) {
	normalized := cfg
	warnings := make([]string, 0, 2)

	level := strings.ToLower(strings.TrimSpace(cfg.Level))
	switch level {
	case "", config.LogLevelInfo, config.LogLevelDebug, config.LogLevelWarn, config.LogLevelError:
	default:
		warnings = append(warnings, "invalid logging.level "+quote(cfg.Level)+", using "+quote(config.LogLevelInfo))
	}

	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	switch format {
	case "", config.LogFormatJSON, config.LogFormatText:
	default:
		warnings = append(warnings, "invalid logging.format "+quote(cfg.Format)+", using "+quote(config.LogFormatJSON))
	}

	normalized.Normalize()
	return normalized, warnings
}

func levelFromString(level string) slog.Level {
	switch level {
	case config.LogLevelDebug:
		return slog.LevelDebug
	case config.LogLevelWarn:
		return slog.LevelWarn
	case config.LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func quote(value string) string {
	if strings.TrimSpace(value) == "" {
		return `""`
	}
	return `"` + strings.TrimSpace(value) + `"`
}

// LevelWriter bridges plain text writers into structured logs.
type LevelWriter struct {
	logger *slog.Logger
	level  slog.Level

	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *LevelWriter) Write(p []byte) (int, error) {
	if w == nil || w.logger == nil {
		return len(p), nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.buf.Write(p); err != nil {
		return 0, err
	}

	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				w.buf.WriteString(line)
				return len(p), nil
			}
			return 0, err
		}
		w.logLine(line)
	}
}

func (w *LevelWriter) Flush() {
	if w == nil || w.logger == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf.Len() == 0 {
		return
	}
	line := w.buf.String()
	w.buf.Reset()
	w.logLine(line)
}

func (w *LevelWriter) logLine(line string) {
	msg := strings.TrimSpace(line)
	if msg == "" {
		return
	}
	w.logger.Log(nil, w.level, msg)
}

// SimpleTextHandler formats logs as concise operator-friendly lines:
// LEVEL component: message key=value ...
type SimpleTextHandler struct {
	writer io.Writer
	opts   slog.HandlerOptions
	attrs  []slog.Attr
	group  string
	mu     *sync.Mutex
}

func NewSimpleTextHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	handlerOpts := slog.HandlerOptions{}
	if opts != nil {
		handlerOpts = *opts
	}
	return &SimpleTextHandler{
		writer: w,
		opts:   handlerOpts,
		mu:     &sync.Mutex{},
	}
}

func (h *SimpleTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		return level >= h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *SimpleTextHandler) Handle(_ context.Context, record slog.Record) error {
	var line strings.Builder
	line.WriteString(strings.ToUpper(record.Level.String()))

	attrs := make([]slog.Attr, 0, len(h.attrs)+record.NumAttrs())
	attrs = append(attrs, h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	component := ""
	for _, attr := range attrs {
		key := h.attrKey(attr.Key)
		if key == "component" {
			component = formatAttrValue(attr.Value)
			break
		}
	}
	if component != "" {
		line.WriteByte(' ')
		line.WriteString(component)
	}
	line.WriteString(": ")
	line.WriteString(record.Message)

	for _, attr := range attrs {
		key := h.attrKey(attr.Key)
		if key == "component" || key == "event" {
			continue
		}
		line.WriteByte(' ')
		line.WriteString(key)
		line.WriteByte('=')
		line.WriteString(formatAttrValue(attr.Value))
	}
	line.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.writer, line.String())
	return err
}

func (h *SimpleTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := *h
	cloned.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cloned
}

func (h *SimpleTextHandler) WithGroup(name string) slog.Handler {
	cloned := *h
	if h.group == "" {
		cloned.group = name
	} else {
		cloned.group = h.group + "." + name
	}
	return &cloned
}

func (h *SimpleTextHandler) attrKey(key string) string {
	if h.group == "" {
		return key
	}
	return h.group + "." + key
}

func formatAttrValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if strings.ContainsAny(s, " \t\n\"") {
			return strconv.Quote(s)
		}
		return s
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'f', -1, 64)
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format("2006-01-02T15:04:05Z07:00")
	case slog.KindAny:
		return formatAnyValue(v.Any())
	default:
		s := v.String()
		if s == "" {
			return formatAnyValue(v.Any())
		}
		return s
	}
}

func formatAnyValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case error:
		return strconv.Quote(x.Error())
	case string:
		if strings.ContainsAny(x, " \t\n\"") {
			return strconv.Quote(x)
		}
		return x
	default:
		s := strings.ReplaceAll(strings.TrimSpace(fmt.Sprint(x)), "\n", "\\n")
		if s == "" {
			return `""`
		}
		if strings.ContainsAny(s, " \t\"") {
			return strconv.Quote(s)
		}
		return s
	}
}
