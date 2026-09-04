package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

// validateResponseKind enforces the invariants between Kind and
// CollectionResponse/Body for a create or update payload. op is the owning
// operation and statusCode the response's (possibly just-updated) status
// code, both used to cross-check the collection response against the spec's
// response schema. Returns a non-empty list of field-specific errors when
// invalid.
func (h *Handler) validateResponseKind(op *models.Operation, statusCode int, kind models.ResponseKind, body string, cr *models.CollectionResponseConfig) []string {
	if kind != models.ResponseKindCollection {
		if cr != nil {
			return []string{`collectionResponse is only allowed when kind is "collection"`}
		}
		return nil
	}

	var errs []string
	if strings.TrimSpace(body) != "" {
		errs = append(errs, "a collection response cannot have a manual body value")
	}
	if cr == nil {
		return append(errs, `collectionResponse is required when kind is "collection"`)
	}
	errs = append(errs, cr.Validate()...)
	if len(errs) > 0 {
		return errs
	}

	if h.collResponseSvc == nil {
		return []string{"collection responses require a collection backend to be configured"}
	}
	crossErrs, err := h.collResponseSvc.ValidateAgainstOperation(op, &models.ResponseConfig{StatusCode: statusCode, CollectionResponse: cr})
	if err != nil {
		return []string{"could not validate collection response against the operation's spec: " + err.Error()}
	}
	return crossErrs
}

// ListResponseConfigs returns all response configs for an operation
func (h *Handler) ListResponseConfigs(c *gin.Context) {
	opID := c.Param("id")

	configs, err := h.store.GetResponseConfigsByOperation(opID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, configs)
}

// CreateResponseConfig creates a new response config
func (h *Handler) CreateResponseConfig(c *gin.Context) {
	opID := c.Param("id")

	// Verify operation exists
	op, err := h.store.GetOperation(opID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation not found"})
		return
	}

	var input models.ResponseConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kind := input.Kind
	if kind == "" {
		kind = models.ResponseKindManual
	}

	// Generate ID
	cfg := &models.ResponseConfig{
		ID:                 generateID(),
		OperationID:        opID,
		Name:               input.Name,
		Description:        input.Description,
		Tag:                normalizeTag(input.Tag),
		Priority:           input.Priority,
		Conditions:         input.Conditions,
		ConditionTree:      input.ConditionTree,
		StatusCode:         input.StatusCode,
		Headers:            input.Headers,
		Body:               input.Body,
		Delay:              input.Delay,
		Enabled:            input.Enabled,
		Kind:               kind,
		CollectionResponse: input.CollectionResponse,
	}

	if cfg.Tag == "" {
		cfg.Tag = models.DefaultTagName
	}
	if err := h.ensureTagExists(cfg.Tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if cfg.StatusCode == 0 {
		cfg.StatusCode = 200
	}
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}
	if cfg.Conditions == nil {
		cfg.Conditions = make([]models.Condition, 0)
	}

	if errs := h.validateResponseKind(op, cfg.StatusCode, kind, cfg.Body, cfg.CollectionResponse); len(errs) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": strings.Join(errs, "; "), "fieldErrors": errs})
		return
	}
	if kind == models.ResponseKindCollection {
		cfg.Body = ""
	}

	if err := h.store.CreateResponseConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cfg)
}

// GetResponseConfig returns a single response config
func (h *Handler) GetResponseConfig(c *gin.Context) {
	id := c.Param("id")

	cfg, err := h.store.GetResponseConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	c.JSON(http.StatusOK, cfg)
}

// UpdateResponseConfig updates a response config
func (h *Handler) UpdateResponseConfig(c *gin.Context) {
	id := c.Param("id")

	cfg, err := h.store.GetResponseConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	var update models.ResponseConfigUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply updates
	if update.Name != nil {
		cfg.Name = *update.Name
	}
	if update.Description != nil {
		cfg.Description = *update.Description
	}
	if update.Tag != nil {
		cfg.Tag = normalizeTag(*update.Tag)
		if cfg.Tag == "" {
			cfg.Tag = models.DefaultTagName
		}
		if err := h.ensureTagExists(cfg.Tag); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if update.Priority != nil {
		cfg.Priority = *update.Priority
	}
	if update.Conditions != nil {
		cfg.Conditions = *update.Conditions
	}
	if update.ConditionTree != nil {
		cfg.ConditionTree = update.ConditionTree
	}
	if update.StatusCode != nil {
		cfg.StatusCode = *update.StatusCode
	}
	if update.Headers != nil {
		cfg.Headers = *update.Headers
	}
	if update.Body != nil {
		cfg.Body = *update.Body
	}
	if update.Delay != nil {
		cfg.Delay = *update.Delay
	}
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.CollectionResponse != nil {
		if !cfg.IsCollectionResponse() {
			c.JSON(http.StatusBadRequest, gin.H{"error": `collectionResponse can only be updated on a response created with kind "collection"`})
			return
		}
		cfg.CollectionResponse = update.CollectionResponse
	}

	if cfg.IsCollectionResponse() {
		op, err := h.store.GetOperation(cfg.OperationID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if errs := h.validateResponseKind(op, cfg.StatusCode, models.ResponseKindCollection, cfg.Body, cfg.CollectionResponse); len(errs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": strings.Join(errs, "; "), "fieldErrors": errs})
			return
		}
		cfg.Body = ""
	}

	if err := h.store.UpdateResponseConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cfg)
}

// DeleteResponseConfig deletes a response config
func (h *Handler) DeleteResponseConfig(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.DeleteResponseConfig(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Response config deleted"})
}

// CloneResponseConfig deep-copies a response config as a new manual response.
// The clone has origin="manual", recorded=false, and signature conditions stripped.
func (h *Handler) CloneResponseConfig(c *gin.Context) {
	id := c.Param("id")

	src, err := h.store.GetResponseConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	var input struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Deep-copy conditions, skipping auto-generated signature conditions
	clonedConditions := make([]models.Condition, 0, len(src.Conditions))
	for _, cond := range src.Conditions {
		if cond.Source == "signature" {
			continue
		}
		clonedConditions = append(clonedConditions, cond)
	}

	// Copy headers
	clonedHeaders := make(map[string]string, len(src.Headers))
	for k, v := range src.Headers {
		clonedHeaders[k] = v
	}

	clone := &models.ResponseConfig{
		ID:                 generateID(),
		OperationID:        src.OperationID,
		Name:               input.Name,
		Description:        src.Description,
		Tag:                src.Tag,
		Priority:           src.Priority,
		Conditions:         clonedConditions,
		ConditionTree:      cloneConditionTree(src.ConditionTree),
		StatusCode:         src.StatusCode,
		Headers:            clonedHeaders,
		Body:               src.Body,
		Delay:              src.Delay,
		Enabled:            src.Enabled,
		Origin:             models.ResponseOriginManual,
		Recorded:           false,
		Kind:               src.Kind,
		CollectionResponse: cloneCollectionResponseConfig(src.CollectionResponse),
	}

	if err := h.store.CreateResponseConfig(clone); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, clone)
}

// cloneCollectionResponseConfig deep-copies a CollectionResponseConfig via a
// JSON round-trip, which is sufficient since every field is JSON-serialisable
// data (no pointers requiring identity preservation besides the FallbackToExample bool).
func cloneCollectionResponseConfig(src *models.CollectionResponseConfig) *models.CollectionResponseConfig {
	if src == nil {
		return nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return nil
	}
	var dst models.CollectionResponseConfig
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil
	}
	return &dst
}

// cloneConditionTree deep-copies a ConditionNode tree, stripping signature leaf nodes.
func cloneConditionTree(node *models.ConditionNode) *models.ConditionNode {
	if node == nil {
		return nil
	}
	if node.Condition != nil {
		if node.Condition.Source == "signature" {
			return nil
		}
		c := *node.Condition
		return &models.ConditionNode{Condition: &c}
	}
	var children []*models.ConditionNode
	for _, child := range node.Children {
		if cloned := cloneConditionTree(child); cloned != nil {
			children = append(children, cloned)
		}
	}
	if len(children) == 0 {
		return nil
	}
	return &models.ConditionNode{Operator: node.Operator, Children: children}
}

func (h *Handler) UpdateResponsePriority(c *gin.Context) {
	id := c.Param("id")

	cfg, err := h.store.GetResponseConfig(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Response config not found"})
		return
	}

	var input struct {
		Priority int `json:"priority"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg.Priority = input.Priority

	if err := h.store.UpdateResponseConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cfg)
}
