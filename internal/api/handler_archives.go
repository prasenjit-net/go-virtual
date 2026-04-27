package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/logging"
)

// ArchiveInfo returns the active archive mode so the UI can adapt.
func (h *Handler) ArchiveInfo(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mode": string(h.archiveManager.Mode())})
}

// ── Full-mode endpoints ───────────────────────────────────────────────────────

// ListArchives returns metadata for all stored archives (full mode only).
func (h *Handler) ListArchives(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeFull {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "archive list is not available in snapshot mode; use GET /_api/archives/snapshot to download the current state",
			"mode":  string(h.archiveManager.Mode()),
		})
		return
	}
	list := h.archiveManager.List()
	if list == nil {
		list = []*archive.ArchiveMeta{}
	}
	c.JSON(http.StatusOK, list)
}

// CreateArchive creates a new archive of the current state (full mode only).
func (h *Handler) CreateArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeFull {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "archive creation is not available in snapshot mode; use GET /_api/archives/snapshot to download the current state",
			"mode":  string(h.archiveManager.Mode()),
		})
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

// GetArchive returns metadata for a single archive (full mode only).
func (h *Handler) GetArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeFull {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "individual archive lookup is not available in snapshot mode",
			"mode":  string(h.archiveManager.Mode()),
		})
		return
	}
	meta, err := h.archiveManager.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, meta)
}

// DeleteArchive removes an archive from disk (full mode only).
func (h *Handler) DeleteArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeFull {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "archive deletion is not available in snapshot mode",
			"mode":  string(h.archiveManager.Mode()),
		})
		return
	}
	if err := h.archiveManager.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// DownloadArchive streams the ZIP file for a named archive (full mode only).
func (h *Handler) DownloadArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeFull {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "per-archive download is not available in snapshot mode; use GET /_api/archives/snapshot",
			"mode":  string(h.archiveManager.Mode()),
		})
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

// UploadArchive accepts a multipart ZIP upload and saves it server-side (full mode only).
func (h *Handler) UploadArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeFull {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "archive upload is not available in snapshot mode; use POST /_api/archives/snapshot/restore to upload and restore in one step",
			"mode":  string(h.archiveManager.Mode()),
		})
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

// RestoreArchive applies a stored archive to the live data store (full mode only).
func (h *Handler) RestoreArchive(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeFull {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "per-archive restore is not available in snapshot mode; use POST /_api/archives/snapshot/restore",
			"mode":  string(h.archiveManager.Mode()),
		})
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

// ── Snapshot-mode endpoints ───────────────────────────────────────────────────

// DownloadSnapshot generates a ZIP of the current state and streams it (snapshot mode only).
func (h *Handler) DownloadSnapshot(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeSnapshot {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "snapshot download is not available in full mode; use GET /_api/archives/:id/download",
			"mode":  string(h.archiveManager.Mode()),
		})
		return
	}

	zipBytes, meta, err := h.archiveManager.DownloadSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", `attachment; filename="`+meta.Filename+`"`)
	c.Header("Content-Type", "application/zip")
	c.Data(http.StatusOK, "application/zip", zipBytes)
}

// RestoreSnapshot accepts a multipart ZIP upload and immediately wipes + applies it (snapshot mode only).
func (h *Handler) RestoreSnapshot(c *gin.Context) {
	if h.archiveManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "archive manager not enabled"})
		return
	}
	if h.archiveManager.Mode() != archive.ModeSnapshot {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "snapshot restore is not available in full mode; use POST /_api/archives/:id/restore",
			"mode":  string(h.archiveManager.Mode()),
		})
		return
	}

	const maxBytes = 50 << 20 // 50 MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	if err := c.Request.ParseMultipartForm(maxBytes); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "archive too large (max 50 MB)"})
		return
	}

	file, _, err := c.Request.FormFile("archive")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "archive file is required"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read archive: " + err.Error()})
		return
	}

	resp, err := h.archiveManager.RestoreSnapshot(data)
	if err != nil {
		logging.Logger("api.archive").Error("Snapshot restore failed",
			"event", "snapshot_restore_request_failed",
			"error", err,
		)
		status := http.StatusInternalServerError
		if resp != nil && resp.Result != nil && len(resp.Result.Errors) > 0 {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, gin.H{"error": err.Error(), "result": resp})
		return
	}

	if err := h.proxyEngine.ReloadRoutes(); err != nil {
		logging.Logger("api.archive").Error("Snapshot restore succeeded but route reload failed",
			"event", "snapshot_restore_route_reload_failed",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "result": resp})
		return
	}

	logging.Logger("api.archive").Info("Snapshot restore completed",
		"event", "snapshot_restore_request_completed",
	)
	c.JSON(http.StatusOK, resp)
}

