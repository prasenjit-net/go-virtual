package api

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
)

func (h *Handler) SaveSpecWorkspace(c *gin.Context) {
	h.workspaceMu.Lock()
	defer h.workspaceMu.Unlock()
	var draft models.SpecWorkspace
	if err := c.ShouldBindJSON(&draft); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	current, err := h.buildSpecWorkspace(c.Param("id"))
	if err != nil {
		if err == errWorkspaceSpecNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Spec not found"})
		} else {
			workspaceError(c, err)
		}
		return
	}
	expected := strings.Trim(strings.TrimSpace(c.GetHeader("If-Match")), `"`)
	if expected == "" {
		expected = draft.Revision
	}
	if expected == "" {
		c.JSON(http.StatusPreconditionRequired, gin.H{"error": "workspace revision is required"})
		return
	}
	if expected != current.Revision {
		c.JSON(http.StatusConflict, gin.H{"error": "spec workspace changed on the server", "revision": current.Revision})
		return
	}
	if draft.Spec == nil || draft.Spec.ID != current.Spec.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace spec identity cannot be changed"})
		return
	}
	idMap, err := h.normalizeWorkspaceDraft(&draft, current)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.ApplySpecWorkspace(&draft); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace save failed; the previous workspace is unchanged", "details": err.Error()})
		return
	}
	if h.proxyEngine != nil {
		h.proxyEngine.ReloadRoutes()
	}
	saved, err := h.buildSpecWorkspace(draft.Spec.ID)
	if err != nil {
		workspaceError(c, err)
		return
	}
	c.Header("ETag", `"`+saved.Revision+`"`)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"workspace": saved, "idMap": idMap})
}

func (h *Handler) normalizeWorkspaceDraft(draft, current *models.SpecWorkspace) (map[string]string, error) {
	if draft.Spec.Content != current.Spec.Content || draft.Spec.Version != current.Spec.Version || !draft.Spec.CreatedAt.Equal(current.Spec.CreatedAt) {
		return nil, fmt.Errorf("the imported OpenAPI contract and creation metadata are read-only in this designer")
	}
	draft.Spec.Operations = nil
	draft.Spec.AIScenarios = current.Spec.AIScenarios
	draft.Spec.CreatedAt = current.Spec.CreatedAt
	draft.Spec.UpdatedAt = time.Now()
	if err := h.setSpecModePolicy(draft.Spec, draft.Spec.ModePolicy); err != nil {
		return nil, fmt.Errorf("modePolicy: %w", err)
	}
	if err := validateWorkspaceResponseConditions(draft.Spec.ModePolicy.AI.ConditionTree, draft.Spec.ModePolicy.AI.Conditions, "spec.modePolicy.ai"); err != nil {
		return nil, err
	}
	if err := validateWorkspaceResponseConditions(draft.Spec.ModePolicy.Proxy.ConditionTree, draft.Spec.ModePolicy.Proxy.Conditions, "spec.modePolicy.proxy"); err != nil {
		return nil, err
	}

	currentOps := make(map[string]*models.Operation, len(current.Operations))
	for _, op := range current.Operations {
		currentOps[op.Operation.ID] = op.Operation
	}
	if len(draft.Operations) != len(current.Operations) {
		return nil, fmt.Errorf("operations cannot be added or removed from the uploaded contract")
	}
	seenOps := map[string]bool{}
	for i := range draft.Operations {
		op := draft.Operations[i].Operation
		if op == nil || seenOps[op.ID] || currentOps[op.ID] == nil {
			return nil, fmt.Errorf("workspace contains an unknown or duplicate operation")
		}
		seenOps[op.ID] = true
		provided := normalizedWorkspaceOperation(*op)
		stored := normalizedWorkspaceOperation(*currentOps[op.ID])
		if !reflect.DeepEqual(provided, stored) {
			return nil, fmt.Errorf("operation %s is part of the read-only OpenAPI contract", op.ID)
		}
		signature := op.SignatureConfig
		*op = *currentOps[op.ID]
		op.SignatureConfig = signature
		if signature != nil {
			signature.Normalize()
		}
	}

	currentResponses := map[string]string{}
	currentBindings := map[string]string{}
	currentMappings := map[string]string{}
	currentValidations := map[string]string{}
	addOwned := func(id, owner string, target map[string]string) {
		target[id] = owner
	}
	for _, b := range current.SpecScripts {
		addOwned(b.ID, "spec", currentBindings)
	}
	for _, m := range current.SpecMappings {
		addOwned(m.ID, "spec", currentMappings)
	}
	for _, v := range current.SpecValidations {
		addOwned(v.ID, "spec", currentValidations)
	}
	for _, op := range current.Operations {
		owner := "operation:" + op.Operation.ID
		for _, b := range op.Scripts {
			addOwned(b.ID, owner, currentBindings)
		}
		for _, m := range op.Mappings {
			addOwned(m.ID, owner, currentMappings)
		}
		for _, v := range op.Validations {
			addOwned(v.ID, owner, currentValidations)
		}
		for _, r := range op.Responses {
			rOwner := "response:" + r.Response.ID
			currentResponses[r.Response.ID] = op.Operation.ID
			for _, b := range r.Scripts {
				addOwned(b.ID, rOwner, currentBindings)
			}
			for _, m := range r.Mappings {
				addOwned(m.ID, rOwner, currentMappings)
			}
		}
	}
	idMap := map[string]string{}
	seenBindings, seenMappings, seenValidations, seenResponses := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	assign := func(id, owner string, known map[string]string, seen map[string]bool) (string, error) {
		if id == "" {
			return "", fmt.Errorf("item ID is required")
		}
		if seen[id] {
			return "", fmt.Errorf("duplicate item ID %q", id)
		}
		seen[id] = true
		if isDraftWorkspaceID(id) {
			next := generateID()
			idMap[id] = next
			return next, nil
		}
		if known[id] != owner {
			return "", fmt.Errorf("item %q does not belong to %s", id, owner)
		}
		return id, nil
	}
	for i := range draft.SpecScripts {
		b := draft.SpecScripts[i]
		if b == nil {
			return nil, fmt.Errorf("spec script binding is required")
		}
		id, err := assign(b.ID, "spec", currentBindings, seenBindings)
		if err != nil {
			return nil, err
		}
		b.ID, b.SpecID, b.OperationID, b.ResponseConfigID = id, draft.Spec.ID, "", ""
		if err := validateWorkspaceBinding(b); err != nil {
			return nil, fmt.Errorf("specScripts[%s]: %w", b.ID, err)
		}
		if err := h.validateWorkspaceScriptID(b.ScriptID); err != nil {
			return nil, err
		}
	}
	for i := range draft.SpecMappings {
		m := draft.SpecMappings[i]
		if err := validateWorkspaceMapping(m); err != nil {
			return nil, fmt.Errorf("specMappings[%d]: %w", i, err)
		}
		id, err := assign(m.ID, "spec", currentMappings, seenMappings)
		if err != nil {
			return nil, err
		}
		m.ID, m.SpecID, m.OperationID, m.ResponseConfigID = id, draft.Spec.ID, "", ""
	}
	for i := range draft.SpecValidations {
		v := draft.SpecValidations[i]
		if err := validateWorkspaceValidation(v); err != nil {
			return nil, fmt.Errorf("specValidations[%s]: %w", v.ID, err)
		}
		if err := validateWorkspaceResponseConditions(v.ConditionTree, nil, "specValidations["+v.ID+"].conditionTree"); err != nil {
			return nil, err
		}
		v.UpdatedAt = time.Now()
		id, err := assign(v.ID, "spec", currentValidations, seenValidations)
		if err != nil {
			return nil, err
		}
		v.ID, v.SpecID, v.OperationID = id, draft.Spec.ID, ""
	}
	for i := range draft.Operations {
		op := &draft.Operations[i]
		if op.Operation == nil {
			return nil, fmt.Errorf("operation is required")
		}
		owner := "operation:" + op.Operation.ID
		for j := range op.Scripts {
			b := op.Scripts[j]
			if b == nil {
				return nil, fmt.Errorf("operation %s script binding is required", op.Operation.ID)
			}
			id, err := assign(b.ID, owner, currentBindings, seenBindings)
			if err != nil {
				return nil, err
			}
			b.ID, b.SpecID, b.OperationID, b.ResponseConfigID = id, "", op.Operation.ID, ""
			if err := validateWorkspaceBinding(b); err != nil {
				return nil, fmt.Errorf("operation %s scripts[%s]: %w", op.Operation.ID, b.ID, err)
			}
			if err := h.validateWorkspaceScriptID(b.ScriptID); err != nil {
				return nil, err
			}
		}
		for j := range op.Mappings {
			m := op.Mappings[j]
			if err := validateWorkspaceMapping(m); err != nil {
				return nil, fmt.Errorf("operation %s mappings[%d]: %w", op.Operation.ID, j, err)
			}
			id, err := assign(m.ID, owner, currentMappings, seenMappings)
			if err != nil {
				return nil, err
			}
			m.ID, m.SpecID, m.OperationID, m.ResponseConfigID = id, "", op.Operation.ID, ""
		}
		for j := range op.Validations {
			v := op.Validations[j]
			if err := validateWorkspaceValidation(v); err != nil {
				return nil, fmt.Errorf("operation %s validations[%s]: %w", op.Operation.ID, v.ID, err)
			}
			if err := validateWorkspaceResponseConditions(v.ConditionTree, nil, "operation."+op.Operation.ID+".validations["+v.ID+"].conditionTree"); err != nil {
				return nil, err
			}
			v.UpdatedAt = time.Now()
			id, err := assign(v.ID, owner, currentValidations, seenValidations)
			if err != nil {
				return nil, err
			}
			v.ID, v.SpecID, v.OperationID = id, "", op.Operation.ID
		}
		for j := range op.Responses {
			r := &op.Responses[j]
			if r == nil {
				return nil, fmt.Errorf("response entry is required")
			}
			if r.Response == nil {
				return nil, fmt.Errorf("response is required")
			}
			if r.Response.StatusCode < 100 || r.Response.StatusCode > 599 {
				return nil, fmt.Errorf("response %s statusCode must be between 100 and 599", r.Response.ID)
			}
			if err := validateWorkspaceResponseConditions(r.Response.ConditionTree, r.Response.Conditions, "response."+r.Response.ID+".conditions"); err != nil {
				return nil, err
			}
			if r.Response.Delay < 0 {
				return nil, fmt.Errorf("response %s delay cannot be negative", r.Response.ID)
			}
			old, exists := currentResponses[r.Response.ID]
			responseID, err := assign(r.Response.ID, "operation:"+op.Operation.ID, responseOwnerMap(currentResponses), seenResponses)
			if err != nil {
				return nil, err
			}
			if exists && old != op.Operation.ID {
				return nil, fmt.Errorf("response %s belongs to another operation", r.Response.ID)
			}
			oldID := r.Response.ID
			if exists {
				for _, prevOp := range current.Operations {
					for _, prev := range prevOp.Responses {
						if prev.Response.ID == oldID && prev.Response.EffectiveKind() != r.Response.EffectiveKind() {
							return nil, fmt.Errorf("response kind cannot be changed for existing response %s", oldID)
						}
					}
				}
			}
			r.Response.ID, r.Response.OperationID = responseID, op.Operation.ID
			if !exists {
				r.Response.Origin, r.Response.Recorded = models.ResponseOriginManual, false
			}
			if r.Response.Tag == "" {
				r.Response.Tag = models.DefaultTagName
			}
			if err := h.ensureTagExists(r.Response.Tag); err != nil {
				return nil, err
			}
			if exists {
				for _, prevOp := range current.Operations {
					for _, prev := range prevOp.Responses {
						if prev.Response.ID == oldID {
							r.Response.Origin, r.Response.Recorded = prev.Response.Origin, prev.Response.Recorded
						}
					}
				}
			}
			if r.Response.IsCollectionResponse() {
				if errs := h.validateResponseKind(op.Operation, r.Response.StatusCode, models.ResponseKindCollection, r.Response.Body, r.Response.CollectionResponse); len(errs) != 0 {
					return nil, fmt.Errorf("response %s: %s", responseID, strings.Join(errs, "; "))
				}
				r.Response.Body = ""
			} else if r.Response.CollectionResponse != nil {
				return nil, fmt.Errorf("response %s: collectionResponse is only allowed for collection responses", r.Response.ID)
			}
			responseOwner := "response:" + oldID
			for k := range r.Scripts {
				b := r.Scripts[k]
				if b == nil {
					return nil, fmt.Errorf("response script binding is required")
				}
				bid, err := assign(b.ID, responseOwner, currentBindings, seenBindings)
				if err != nil {
					return nil, err
				}
				b.ID, b.SpecID, b.OperationID, b.ResponseConfigID = bid, "", "", responseID
				if err := validateWorkspaceBinding(b); err != nil {
					return nil, fmt.Errorf("response %s scripts[%s]: %w", r.Response.ID, b.ID, err)
				}
				if err := h.validateWorkspaceScriptID(b.ScriptID); err != nil {
					return nil, err
				}
			}
			for k := range r.Mappings {
				m := r.Mappings[k]
				if m == nil {
					return nil, fmt.Errorf("response collection mapping is required")
				}
				if r.Response.IsCollectionResponse() {
					return nil, fmt.Errorf("response-level collection mappings are not supported for collection responses")
				}
				if err := validateWorkspaceMapping(m); err != nil {
					return nil, fmt.Errorf("response %s mappings[%d]: %w", r.Response.ID, k, err)
				}
				mid, err := assign(m.ID, responseOwner, currentMappings, seenMappings)
				if err != nil {
					return nil, err
				}
				m.ID, m.SpecID, m.OperationID, m.ResponseConfigID = mid, "", "", responseID
			}
			if r.Response.IsCollectionResponse() {
				if errs := r.Response.CollectionResponse.Validate(); len(errs) != 0 {
					return nil, fmt.Errorf("response %s collectionResponse: %s", r.Response.ID, strings.Join(errs, "; "))
				}
			}
		}
	}
	return idMap, nil
}

// normalizedWorkspaceOperation removes values which are configured through
// other workspace collections. Response configs are returned separately in a
// workspace bundle, but a storage implementation may also populate
// Operation.Responses for the normal operation endpoint. That derived list
// must never make an otherwise valid workspace draft look like a contract edit.
func normalizedWorkspaceOperation(operation models.Operation) models.Operation {
	operation.SignatureConfig = nil
	operation.Responses = nil
	if len(operation.Tags) == 0 {
		operation.Tags = nil
	}
	if len(operation.DeclaredPathParams) == 0 {
		operation.DeclaredPathParams = nil
	}
	if len(operation.DeclaredQueryParams) == 0 {
		operation.DeclaredQueryParams = nil
	}
	if len(operation.DeclaredHeaderParams) == 0 {
		operation.DeclaredHeaderParams = nil
	}
	if len(operation.DeclaredBodyFields) == 0 {
		operation.DeclaredBodyFields = nil
	}
	if operation.ExampleResponse != nil && len(operation.ExampleResponse.Headers) == 0 {
		example := *operation.ExampleResponse
		example.Headers = nil
		operation.ExampleResponse = &example
	}
	return operation
}

func isDraftWorkspaceID(id string) bool {
	return strings.HasPrefix(id, "draft-") || strings.HasPrefix(id, "tmp-")
}

func responseOwnerMap(ids map[string]string) map[string]string {
	out := make(map[string]string, len(ids))
	for id, operationID := range ids {
		out[id] = "operation:" + operationID
	}
	return out
}

func (h *Handler) validateWorkspaceScriptID(id string) error {
	if id == "" {
		return fmt.Errorf("scriptId is required")
	}
	if _, err := h.store.GetScript(id); err != nil {
		return fmt.Errorf("script %s does not exist", id)
	}
	return nil
}

var workspaceOutputKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateWorkspaceMapping(mapping *models.CollectionMapping) error {
	if mapping == nil {
		return fmt.Errorf("mapping is required")
	}
	if strings.TrimSpace(mapping.CollectionName) == "" {
		return fmt.Errorf("collectionName is required")
	}
	if !workspaceOutputKey.MatchString(mapping.OutputKey) {
		return fmt.Errorf("outputKey must be a simple identifier")
	}
	switch mapping.Operation {
	case models.ColOpInsert, models.ColOpUpdate, models.ColOpUpsert, models.ColOpDelete, models.ColOpFindOne, models.ColOpFindMany:
	default:
		return fmt.Errorf("unsupported operation %q", mapping.Operation)
	}
	for i, rule := range append(append([]models.FieldMappingRule(nil), mapping.FilterRules...), mapping.DataRules...) {
		if strings.TrimSpace(rule.TargetField) == "" {
			return fmt.Errorf("mapping rule %d targetField is required", i)
		}
		if rule.SkipWhenMissing && rule.DefaultValue != nil {
			return fmt.Errorf("mapping rule %d cannot set both skipWhenMissing and defaultValue", i)
		}
		switch rule.SourceType {
		case "path", "query", "header", "body", "session", "store", "literal":
		default:
			return fmt.Errorf("mapping rule %d has unsupported sourceType %q", i, rule.SourceType)
		}
		if rule.SourceType != "literal" && strings.TrimSpace(rule.SourceKey) == "" {
			return fmt.Errorf("mapping rule %d sourceKey is required", i)
		}
	}
	return nil
}

func validateWorkspaceValidation(rule *models.ValidationRule) error {
	if rule == nil {
		return fmt.Errorf("validation rule is required")
	}
	if !validationNameRe.MatchString(rule.Name) {
		return fmt.Errorf("name must match ^[a-zA-Z_][a-zA-Z0-9_]*$")
	}
	return nil
}

func validateWorkspaceBinding(binding *models.ScriptBinding) error {
	if binding == nil {
		return fmt.Errorf("script binding is required")
	}
	if strings.TrimSpace(binding.OutputKey) == "" {
		return fmt.Errorf("outputKey is required")
	}
	if binding.Order < 0 {
		return fmt.Errorf("order cannot be negative")
	}
	return nil
}

func validateWorkspaceResponseConditions(tree *models.ConditionNode, conditions []models.Condition, path string) error {
	if tree != nil {
		if err := validateWorkspaceConditionTree(tree, path+".conditionTree", 0); err != nil {
			return err
		}
		return nil
	}
	if err := validateConditions(conditions); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(conditions) > 0 && (conditions[0].Source == models.SourceSignature || conditions[0].Source == models.SourceScriptOutput || conditions[0].Source == models.SourceValidation || conditions[0].Source == models.SourceCollectionOutput) {
		return fmt.Errorf("%s legacy conditions cannot use signature, script, validation, or collection sources", path)
	}
	return nil
}

func validateWorkspaceConditionTree(node *models.ConditionNode, path string, depth int) error {
	if node == nil {
		return fmt.Errorf("%s cannot contain a null node", path)
	}
	if depth > 32 {
		return fmt.Errorf("%s exceeds maximum condition depth", path)
	}
	if node.Condition != nil {
		if node.Operator != "" || len(node.Children) != 0 {
			return fmt.Errorf("%s cannot combine a condition with a group", path)
		}
		if err := validateConditions([]models.Condition{*node.Condition}); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if node.Condition.Source == models.SourceSignature || node.Condition.Source == models.SourceScriptOutput || node.Condition.Source == models.SourceValidation || node.Condition.Source == models.SourceCollectionOutput {
			return fmt.Errorf("%s uses a source that is not supported by this condition scope", path)
		}
		return nil
	}
	switch node.Operator {
	case "AND", "OR":
		if len(node.Children) < 2 {
			return fmt.Errorf("%s %s group must have at least two children", path, node.Operator)
		}
	case "NOT":
		if len(node.Children) != 1 {
			return fmt.Errorf("%s NOT group must have exactly one child", path)
		}
	default:
		return fmt.Errorf("%s has invalid group operator %q", path, node.Operator)
	}
	for i, child := range node.Children {
		if err := validateWorkspaceConditionTree(child, fmt.Sprintf("%s.children[%d]", path, i), depth+1); err != nil {
			return err
		}
	}
	return nil
}
