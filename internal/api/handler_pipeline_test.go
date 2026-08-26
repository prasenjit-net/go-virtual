package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
)

func setupPipelineTest(t *testing.T) (*Handler, storage.Storage, *gin.Engine) {
	t.Helper()

	handler, store, r := setupTestHandler(t)
	if err := store.CreateSpec(&models.Spec{ID: "spec-1", Name: "Spec"}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if err := store.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/pets"}); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}
	if err := store.CreateResponseConfig(&models.ResponseConfig{ID: "resp-1", OperationID: "op-1", Name: "OK", StatusCode: 200}); err != nil {
		t.Fatalf("CreateResponseConfig: %v", err)
	}

	return handler, store, r
}

func decodePipelineSteps(t *testing.T, body *bytes.Buffer) []models.PipelineStep {
	t.Helper()

	var payload struct {
		Steps []models.PipelineStep `json:"steps"`
	}
	if err := json.Unmarshal(body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, body.String())
	}
	return payload.Steps
}

func performPipelineRequest(r *gin.Engine, method, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestListPipelineScopes(t *testing.T) {
	handler, store, r := setupPipelineTest(t)
	r.GET("/specs/:id/pipeline", handler.ListSpecPipeline)
	r.GET("/operations/:id/pipeline", handler.ListOperationPipeline)
	r.GET("/responses/:id/pipeline", handler.ListResponsePipeline)

	mustCreateScriptBinding(t, store, &models.ScriptBinding{ID: "spec-script", SpecID: "spec-1", ScriptID: "script-1", OutputKey: "specScript", Order: 20, Enabled: true})
	mustCreateValidationRule(t, store, &models.ValidationRule{ID: "spec-validation", SpecID: "spec-1", Name: "specValidation", Order: 20, Enabled: true})
	mustCreateCollectionMapping(t, store, &models.CollectionMapping{ID: "spec-collection", SpecID: "spec-1", CollectionName: "pets", Operation: models.ColOpFindOne, OutputKey: "specCollection", Order: 10, Enabled: true})

	mustCreateScriptBinding(t, store, &models.ScriptBinding{ID: "op-script", OperationID: "op-1", ScriptID: "script-1", OutputKey: "opScript", Order: 5, Enabled: true})
	mustCreateValidationRule(t, store, &models.ValidationRule{ID: "op-validation", OperationID: "op-1", Name: "opValidation", Order: 5, Enabled: true})
	mustCreateCollectionMapping(t, store, &models.CollectionMapping{ID: "op-collection", OperationID: "op-1", CollectionName: "pets", Operation: models.ColOpFindMany, OutputKey: "opCollection", Order: 5, Enabled: true})

	mustCreateScriptBinding(t, store, &models.ScriptBinding{ID: "resp-script", ResponseConfigID: "resp-1", ScriptID: "script-1", OutputKey: "respScript", Order: 20, Enabled: true})
	mustCreateCollectionMapping(t, store, &models.CollectionMapping{ID: "resp-collection", ResponseConfigID: "resp-1", CollectionName: "pets", Operation: models.ColOpInsert, OutputKey: "respCollection", Order: 10, Enabled: true})

	tests := []struct {
		name      string
		path      string
		wantTypes []models.PipelineStepType
		wantIDs   []string
	}{
		{
			name:      "spec",
			path:      "/specs/spec-1/pipeline",
			wantTypes: []models.PipelineStepType{models.PipelineStepCollection, models.PipelineStepScript, models.PipelineStepValidation},
			wantIDs:   []string{"spec-collection", "spec-script", "spec-validation"},
		},
		{
			name:      "operation ties sort by script validation collection",
			path:      "/operations/op-1/pipeline",
			wantTypes: []models.PipelineStepType{models.PipelineStepScript, models.PipelineStepValidation, models.PipelineStepCollection},
			wantIDs:   []string{"op-script", "op-validation", "op-collection"},
		},
		{
			name:      "response excludes validations",
			path:      "/responses/resp-1/pipeline",
			wantTypes: []models.PipelineStepType{models.PipelineStepCollection, models.PipelineStepScript},
			wantIDs:   []string{"resp-collection", "resp-script"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performPipelineRequest(r, http.MethodGet, tt.path, "")
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			steps := decodePipelineSteps(t, w.Body)
			if len(steps) != len(tt.wantTypes) {
				t.Fatalf("step count=%d want=%d steps=%+v", len(steps), len(tt.wantTypes), steps)
			}
			for i, step := range steps {
				if step.Type != tt.wantTypes[i] {
					t.Fatalf("step[%d] type=%q want=%q", i, step.Type, tt.wantTypes[i])
				}
				if gotID := pipelineStepID(step); gotID != tt.wantIDs[i] {
					t.Fatalf("step[%d] id=%q want=%q", i, gotID, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestListPipelineNotFound(t *testing.T) {
	handler, _, r := setupPipelineTest(t)
	r.GET("/specs/:id/pipeline", handler.ListSpecPipeline)
	r.GET("/operations/:id/pipeline", handler.ListOperationPipeline)
	r.GET("/responses/:id/pipeline", handler.ListResponsePipeline)

	for _, path := range []string{
		"/specs/missing/pipeline",
		"/operations/missing/pipeline",
		"/responses/missing/pipeline",
	} {
		w := performPipelineRequest(r, http.MethodGet, path, "")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d want=%d body=%s", path, w.Code, http.StatusNotFound, w.Body.String())
		}
	}
}

func TestReorderOperationPipelineUpdatesAllStepTypes(t *testing.T) {
	handler, store, r := setupPipelineTest(t)
	r.PUT("/operations/:id/pipeline/reorder", handler.ReorderOperationPipeline)

	mustCreateScriptBinding(t, store, &models.ScriptBinding{ID: "script-1", OperationID: "op-1", ScriptID: "script-1", OutputKey: "script", Order: 90, Enabled: true})
	mustCreateValidationRule(t, store, &models.ValidationRule{ID: "validation-1", OperationID: "op-1", Name: "validation", Order: 80, Enabled: true})
	mustCreateCollectionMapping(t, store, &models.CollectionMapping{ID: "collection-1", OperationID: "op-1", CollectionName: "pets", Operation: models.ColOpFindOne, OutputKey: "collection", Order: 70, Enabled: true})

	body := `[
		{"type":"collection","id":"collection-1"},
		{"type":"validation","id":"validation-1"},
		{"type":"script","id":"script-1"}
	]`
	w := performPipelineRequest(r, http.MethodPut, "/operations/op-1/pipeline/reorder", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	bindings, err := store.GetScriptBindings("op-1")
	if err != nil {
		t.Fatalf("GetScriptBindings: %v", err)
	}
	rule, err := store.GetValidationRule("validation-1")
	if err != nil {
		t.Fatalf("GetValidationRule: %v", err)
	}
	mapping, err := store.GetCollectionMapping("collection-1")
	if err != nil {
		t.Fatalf("GetCollectionMapping: %v", err)
	}

	if mapping.Order != 0 {
		t.Fatalf("collection order=%d want=0", mapping.Order)
	}
	if rule.Order != 10 {
		t.Fatalf("validation order=%d want=10", rule.Order)
	}
	if len(bindings) != 1 || bindings[0].Order != 20 {
		t.Fatalf("script bindings=%+v want one binding with order 20", bindings)
	}
}

func TestReorderSpecPipelineUpdatesAllStepTypes(t *testing.T) {
	handler, store, r := setupPipelineTest(t)
	r.PUT("/specs/:id/pipeline/reorder", handler.ReorderSpecPipeline)

	mustCreateScriptBinding(t, store, &models.ScriptBinding{ID: "spec-script", SpecID: "spec-1", ScriptID: "script-1", OutputKey: "script", Order: 30, Enabled: true})
	mustCreateValidationRule(t, store, &models.ValidationRule{ID: "spec-validation", SpecID: "spec-1", Name: "validation", Order: 20, Enabled: true})
	mustCreateCollectionMapping(t, store, &models.CollectionMapping{ID: "spec-collection", SpecID: "spec-1", CollectionName: "pets", Operation: models.ColOpFindMany, OutputKey: "collection", Order: 10, Enabled: true})

	body := `[
		{"type":"validation","id":"spec-validation"},
		{"type":"script","id":"spec-script"},
		{"type":"collection","id":"spec-collection"}
	]`
	w := performPipelineRequest(r, http.MethodPut, "/specs/spec-1/pipeline/reorder", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	bindings, err := store.GetSpecScriptBindings("spec-1")
	if err != nil {
		t.Fatalf("GetSpecScriptBindings: %v", err)
	}
	rule, err := store.GetValidationRule("spec-validation")
	if err != nil {
		t.Fatalf("GetValidationRule: %v", err)
	}
	mapping, err := store.GetCollectionMapping("spec-collection")
	if err != nil {
		t.Fatalf("GetCollectionMapping: %v", err)
	}

	if rule.Order != 0 {
		t.Fatalf("validation order=%d want=0", rule.Order)
	}
	if len(bindings) != 1 || bindings[0].Order != 10 {
		t.Fatalf("script bindings=%+v want one binding with order 10", bindings)
	}
	if mapping.Order != 20 {
		t.Fatalf("collection order=%d want=20", mapping.Order)
	}
}

func TestReorderPipelineErrors(t *testing.T) {
	handler, store, r := setupPipelineTest(t)
	r.PUT("/specs/:id/pipeline/reorder", handler.ReorderSpecPipeline)
	r.PUT("/operations/:id/pipeline/reorder", handler.ReorderOperationPipeline)
	r.PUT("/responses/:id/pipeline/reorder", handler.ReorderResponsePipeline)

	mustCreateScriptBinding(t, store, &models.ScriptBinding{ID: "resp-script", ResponseConfigID: "resp-1", ScriptID: "script-1", OutputKey: "script", Order: 10, Enabled: true})
	mustCreateCollectionMapping(t, store, &models.CollectionMapping{ID: "resp-collection", ResponseConfigID: "resp-1", CollectionName: "pets", Operation: models.ColOpInsert, OutputKey: "collection", Order: 20, Enabled: true})

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing operation",
			method:     http.MethodPut,
			path:       "/operations/missing/pipeline/reorder",
			body:       `[]`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing spec",
			method:     http.MethodPut,
			path:       "/specs/missing/pipeline/reorder",
			body:       `[]`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "bad json",
			method:     http.MethodPut,
			path:       "/operations/op-1/pipeline/reorder",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown type",
			method:     http.MethodPut,
			path:       "/operations/op-1/pipeline/reorder",
			body:       `[{"type":"bogus","id":"x"}]`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing script binding",
			method:     http.MethodPut,
			path:       "/operations/op-1/pipeline/reorder",
			body:       `[{"type":"script","id":"missing"}]`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing validation rule",
			method:     http.MethodPut,
			path:       "/operations/op-1/pipeline/reorder",
			body:       `[{"type":"validation","id":"missing"}]`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing collection mapping",
			method:     http.MethodPut,
			path:       "/operations/op-1/pipeline/reorder",
			body:       `[{"type":"collection","id":"missing"}]`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "response scope accepts scripts and collections",
			method:     http.MethodPut,
			path:       "/responses/resp-1/pipeline/reorder",
			body:       `[{"type":"script","id":"resp-script"},{"type":"collection","id":"resp-collection"}]`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "response scope rejects validations",
			method:     http.MethodPut,
			path:       "/responses/resp-1/pipeline/reorder",
			body:       `[{"type":"validation","id":"validation-1"}]`,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performPipelineRequest(r, tt.method, tt.path, tt.body)
			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func mustCreateScriptBinding(t *testing.T, store storage.Storage, binding *models.ScriptBinding) {
	t.Helper()
	if err := store.CreateScriptBinding(binding); err != nil {
		t.Fatalf("CreateScriptBinding(%s): %v", binding.ID, err)
	}
}

func mustCreateValidationRule(t *testing.T, store storage.Storage, rule *models.ValidationRule) {
	t.Helper()
	if _, err := store.CreateValidationRule(rule); err != nil {
		t.Fatalf("CreateValidationRule(%s): %v", rule.ID, err)
	}
}

func mustCreateCollectionMapping(t *testing.T, store storage.Storage, mapping *models.CollectionMapping) {
	t.Helper()
	if err := store.CreateCollectionMapping(mapping); err != nil {
		t.Fatalf("CreateCollectionMapping(%s): %v", mapping.ID, err)
	}
}

func pipelineStepID(step models.PipelineStep) string {
	switch step.Type {
	case models.PipelineStepScript:
		if step.Script != nil {
			return step.Script.ID
		}
	case models.PipelineStepValidation:
		if step.Validation != nil {
			return step.Validation.ID
		}
	case models.PipelineStepCollection:
		if step.Collection != nil {
			return step.Collection.ID
		}
	}
	return ""
}
