package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/logging"
)

// ListArchives returns metadata for all stored archives.
func (h *Handler) ListArchives(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	list := h.archiveManager.List()
	if list == nil {
		list = []*archive.ArchiveMeta{}
	}
	c.JSON(http.StatusOK, list)
}

// CreateArchive creates a new archive of the current state.
func (h *Handler) CreateArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	var input struct {
		Label string `json:"label"`
	}
	_ = c.ShouldBindJSON(&input) // label is optional
	meta, err := h.archiveManager.Create(input.Label)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, meta)
}

// GetArchive returns metadata for a single archive.
func (h *Handler) GetArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	meta, err := h.archiveManager.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, meta)
}

// DeleteArchive removes an archive from disk.
func (h *Handler) DeleteArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if err := h.archiveManager.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// DownloadArchive streams the ZIP file as a browser download.
func (h *Handler) DownloadArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	meta, err := h.archiveManager.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	path, err := h.archiveManager.FilePath(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.FileAttachment(path, meta.Filename)
}

// UploadArchive accepts a multipart ZIP upload and saves it server-side.
func (h *Handler) UploadArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}

	const maxBytes = 50 << 20 // 50 MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	if err := c.Request.ParseMultipartForm(maxBytes); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "archive too large (max 50 MB)"})
		return
	}

	file, header, err := c.Request.FormFile("archive")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "archive file is required"})
		return
	}
	defer file.Close()

	label := c.Request.FormValue("label")
	meta, err := h.archiveManager.Save(file, header.Size, label)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, meta)
}

// RestoreArchive applies the archive to the live data store.
func (h *Handler) RestoreArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}

	var input archive.RestoreInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.archiveManager.Restore(c.Param("id"), input)
	if err != nil {
		logging.Logger("api.archive").Error("Archive restore request failed",
			"event", "archive_restore_request_failed",
			"archive_id", c.Param("id"),
			"wipe_first", input.WipeFirst,
			"create_backup_first", input.CreateBackupFirst,
			"error", err,
		)
		status := http.StatusInternalServerError
		if resp != nil && resp.Result != nil && len(resp.Result.Errors) > 0 {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, gin.H{"error": err.Error(), "result": resp})
		return
	}

	// Reload proxy routes so the restored spec registrations take effect.
	if err := h.proxyEngine.ReloadRoutes(); err != nil {
		logging.Logger("api.archive").Error("Archive restore succeeded but route reload failed",
			"event", "archive_restore_route_reload_failed",
			"archive_id", c.Param("id"),
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "result": resp})
		return
	}
	logging.Logger("api.archive").Info("Archive restore request completed",
		"event", "archive_restore_request_completed",
		"archive_id", c.Param("id"),
		"wipe_first", input.WipeFirst,
		"create_backup_first", input.CreateBackupFirst,
	)

	c.JSON(http.StatusOK, resp)
}
