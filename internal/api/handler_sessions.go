package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// ListSessions returns metadata for all active sessions.
func (h *Handler) ListSessions(c *gin.Context) {
	if h.sessionManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sessions not enabled"})
		return
	}

	infos, err := h.sessionManager.ActiveSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if infos == nil {
		infos = []models.SessionInfo{}
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": infos,
		"count":    len(infos),
	})
}

// GetSession returns metadata and store snapshot for a single session.
func (h *Handler) GetSession(c *gin.Context) {
	if h.sessionManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sessions not enabled"})
		return
	}

	id := c.Param("id")
	sess, ok, err := h.sessionManager.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, sess.Info(true))
}

// InvalidateSession removes a single session.
func (h *Handler) InvalidateSession(c *gin.Context) {
	if h.sessionManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sessions not enabled"})
		return
	}

	id := c.Param("id")
	if err := h.sessionManager.Invalidate(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// InvalidateAllSessions removes all sessions.
// Requires query param ?confirm=true.
func (h *Handler) InvalidateAllSessions(c *gin.Context) {
	if h.sessionManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sessions not enabled"})
		return
	}

	if c.Query("confirm") != "true" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pass ?confirm=true to invalidate all sessions"})
		return
	}

	if err := h.sessionManager.InvalidateAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
