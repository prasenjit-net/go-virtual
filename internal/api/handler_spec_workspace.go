package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

type SpecWorkspace = models.SpecWorkspace
type WorkspaceOperation = models.SpecWorkspaceOperation
type WorkspaceResponse = models.SpecWorkspaceResponse

var errWorkspaceSpecNotFound = errors.New("spec not found")

// GetSpecWorkspace returns one read-only snapshot for the designer.
func (h *Handler) GetSpecWorkspace(c *gin.Context) {
	workspace, err := h.buildSpecWorkspace(c.Param("id"))
	if err != nil {
		if errors.Is(err, errWorkspaceSpecNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		} else {
			workspaceError(c, err)
		}
		return
	}
	c.Header("ETag", `"`+workspace.Revision+`"`)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, workspace)
}

func (h *Handler) buildSpecWorkspace(specID string) (*models.SpecWorkspace, error) {
	spec, err := h.store.GetSpec(specID)
	if err != nil {
		return nil, errWorkspaceSpecNotFound
	}
	// Keep the imported OpenAPI document on the spec and avoid duplicating
	// operations in the bundle. Operation entities are returned separately.
	specCopy := *spec
	specCopy.Operations = nil
	spec = &specCopy
	workspace := SpecWorkspace{Spec: spec, Operations: []WorkspaceOperation{}}
	workspace.SpecScripts, err = h.store.GetSpecScriptBindings(spec.ID)
	if err != nil {
		return nil, err
	}
	workspace.SpecValidations, err = h.store.ListValidationRulesBySpec(spec.ID)
	if err != nil {
		return nil, err
	}
	workspace.SpecMappings, err = h.store.GetCollectionMappingsBySpec(spec.ID)
	if err != nil {
		return nil, err
	}
	sort.Slice(workspace.SpecScripts, func(i, j int) bool {
		return orderedID(workspace.SpecScripts[i].Order, workspace.SpecScripts[i].ID, workspace.SpecScripts[j].Order, workspace.SpecScripts[j].ID)
	})
	sort.Slice(workspace.SpecValidations, func(i, j int) bool {
		return orderedID(workspace.SpecValidations[i].Order, workspace.SpecValidations[i].ID, workspace.SpecValidations[j].Order, workspace.SpecValidations[j].ID)
	})
	sort.Slice(workspace.SpecMappings, func(i, j int) bool {
		return orderedID(workspace.SpecMappings[i].Order, workspace.SpecMappings[i].ID, workspace.SpecMappings[j].Order, workspace.SpecMappings[j].ID)
	})
	ops, err := h.store.GetOperationsBySpec(spec.ID)
	if err != nil {
		return nil, err
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path == ops[j].Path {
			return ops[i].Method < ops[j].Method
		}
		return ops[i].Path < ops[j].Path
	})
	for _, op := range ops {
		item := WorkspaceOperation{Operation: op}
		item.Scripts, err = h.store.GetScriptBindings(op.ID)
		if err != nil {
			return nil, err
		}
		item.Validations, err = h.store.ListValidationRulesByOperation(op.ID)
		if err != nil {
			return nil, err
		}
		item.Mappings, err = h.store.GetCollectionMappingsByOperation(op.ID)
		if err != nil {
			return nil, err
		}
		sort.Slice(item.Scripts, func(i, j int) bool {
			return orderedID(item.Scripts[i].Order, item.Scripts[i].ID, item.Scripts[j].Order, item.Scripts[j].ID)
		})
		sort.Slice(item.Validations, func(i, j int) bool {
			return orderedID(item.Validations[i].Order, item.Validations[i].ID, item.Validations[j].Order, item.Validations[j].ID)
		})
		sort.Slice(item.Mappings, func(i, j int) bool {
			return orderedID(item.Mappings[i].Order, item.Mappings[i].ID, item.Mappings[j].Order, item.Mappings[j].ID)
		})
		responses, err := h.store.GetResponseConfigsByOperation(op.ID)
		if err != nil {
			return nil, err
		}
		sort.Slice(responses, func(i, j int) bool {
			return orderedID(responses[i].Priority, responses[i].ID, responses[j].Priority, responses[j].ID)
		})
		for _, response := range responses {
			child := WorkspaceResponse{Response: response}
			child.Scripts, err = h.store.GetResponseScriptBindings(response.ID)
			if err != nil {
				return nil, err
			}
			child.Mappings, err = h.store.GetCollectionMappingsByResponse(response.ID)
			if err != nil {
				return nil, err
			}
			sort.Slice(child.Scripts, func(i, j int) bool {
				return orderedID(child.Scripts[i].Order, child.Scripts[i].ID, child.Scripts[j].Order, child.Scripts[j].ID)
			})
			sort.Slice(child.Mappings, func(i, j int) bool {
				return orderedID(child.Mappings[i].Order, child.Mappings[i].ID, child.Mappings[j].Order, child.Mappings[j].ID)
			})
			item.Responses = append(item.Responses, child)
		}
		if item.Responses == nil {
			item.Responses = []WorkspaceResponse{}
		}
		workspace.Operations = append(workspace.Operations, item)
	}
	if workspace.SpecScripts == nil {
		workspace.SpecScripts = []*models.ScriptBinding{}
	}
	if workspace.SpecValidations == nil {
		workspace.SpecValidations = []*models.ValidationRule{}
	}
	if workspace.SpecMappings == nil {
		workspace.SpecMappings = []*models.CollectionMapping{}
	}
	if workspace.Operations == nil {
		workspace.Operations = []WorkspaceOperation{}
	}
	canonical, err := json.Marshal(workspace)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(canonical)
	workspace.Revision = hex.EncodeToString(digest[:])
	return &workspace, nil
}

func workspaceError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load spec workspace", "details": err.Error()})
}

func orderedID(orderA int, idA string, orderB int, idB string) bool {
	if orderA == orderB {
		return idA < idB
	}
	return orderA < orderB
}
