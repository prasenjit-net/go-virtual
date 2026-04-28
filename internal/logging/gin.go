package logging

import (
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessMiddleware emits structured request logs with level selection based on
// response status and route type.
func AccessMiddleware() gin.HandlerFunc {
	logger := Logger("http.access")

	return func(c *gin.Context) {
		start := time.Now()
		requestPath := c.Request.URL.Path

		c.Next()

		status := c.Writer.Status()
		duration := time.Since(start)
		route := c.FullPath()
		area := classifyArea(requestPath)
		level := accessLevel(area, requestPath, status)

		attrs := []any{
			"event", "http_request",
			"method", c.Request.Method,
			"path", requestPath,
			"route", routeOrPath(route, requestPath),
			"area", area,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"client_ip", c.ClientIP(),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		logger.Log(c.Request.Context(), level, "HTTP request completed", attrs...)
	}
}

// RecoveryMiddleware logs panics with structured context before returning 500.
func RecoveryMiddleware() gin.HandlerFunc {
	logger := Logger("http.recovery")
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Error("HTTP panic recovered",
			"event", "http_panic",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"client_ip", c.ClientIP(),
			"panic", fmt.Sprint(recovered),
			"stack", string(debug.Stack()),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

func routeOrPath(route, requestPath string) string {
	if route != "" {
		return route
	}
	return requestPath
}

func accessLevel(area, requestPath string, status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	case requestPath == "/_api/health" || requestPath == "/_prometheus":
		return slog.LevelDebug
	case area == "ui-asset" || area == "docs-asset":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

func classifyArea(requestPath string) string {
	switch {
	case requestPath == "/_prometheus":
		return "metrics"
	case strings.HasPrefix(requestPath, "/_api/"):
		return "admin-api"
	case strings.HasPrefix(requestPath, "/_ui/"):
		if hasStaticExtension(requestPath) {
			return "ui-asset"
		}
		return "ui"
	case strings.HasPrefix(requestPath, "/_docs/"):
		if hasStaticExtension(requestPath) {
			return "docs-asset"
		}
		return "docs"
	default:
		return "proxy"
	}
}

func hasStaticExtension(requestPath string) bool {
	ext := strings.ToLower(path.Ext(requestPath))
	switch ext {
	case ".css", ".js", ".map", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".html", ".txt", ".woff", ".woff2":
		return true
	default:
		return false
	}
}
