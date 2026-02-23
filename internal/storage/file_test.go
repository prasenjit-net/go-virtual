package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
)

func TestFileStorage_DefaultTagAndDirs(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage error: %v", err)
	}

	tags, err := fs.ListTags()
	if err != nil {
		t.Fatalf("ListTags error: %v", err)
	}

	foundDefault := false
	for _, tag := range tags {
		if tag.Name == models.DefaultTagName {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		t.Fatalf("expected default tag %q to exist", models.DefaultTagName)
	}

	paths := []string{
		filepath.Join(baseDir, "specs"),
		filepath.Join(baseDir, "responses"),
		filepath.Join(baseDir, "tags.json"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected path to exist: %s (%v)", p, err)
		}
	}

	tagsData, err := os.ReadFile(filepath.Join(baseDir, "tags.json"))
	if err != nil {
		t.Fatalf("read tags.json error: %v", err)
	}
	if !strings.Contains(string(tagsData), models.DefaultTagName) {
		t.Fatalf("expected tags.json to contain default tag %q", models.DefaultTagName)
	}
}

func TestFileStorage_SaveAndReload(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage error: %v", err)
	}

	specContent := `{"openapi":"3.0.0","info":{"title":"Test API","version":"1.0.0"},"paths":{}}`
	now := time.Now()
	spec := &models.Spec{
		ID:                 "spec-1",
		Name:               "Test API",
		Version:            "1.0.0",
		Description:        "desc",
		Content:            specContent,
		BasePath:           "/api",
		Enabled:            true,
		UseExampleFallback: true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := fs.CreateSpec(spec); err != nil {
		t.Fatalf("CreateSpec error: %v", err)
	}

	specMetaPath := filepath.Join(baseDir, "specs", "spec-1.json")
	specContentPath := filepath.Join(baseDir, "specs", "spec-1.spec.json")
	if _, err := os.Stat(specMetaPath); err != nil {
		t.Fatalf("expected spec metadata file: %v", err)
	}
	if _, err := os.Stat(specContentPath); err != nil {
		t.Fatalf("expected spec content file: %v", err)
	}

	respBody := `{"ok":true}`
	cfg := &models.ResponseConfig{
		ID:          "resp-1",
		OperationID: "op-1",
		Name:        "OK",
		Tag:         "",
		Priority:    0,
		StatusCode:  200,
		Headers:     map[string]string{"X-Test": "1"},
		Body:        respBody,
		Enabled:     true,
	}

	if err := fs.CreateResponseConfig(cfg); err != nil {
		t.Fatalf("CreateResponseConfig error: %v", err)
	}

	respMetaPath := filepath.Join(baseDir, "responses", "resp-1.json")
	respBodyPath := filepath.Join(baseDir, "responses", "resp-1.body")
	if _, err := os.Stat(respMetaPath); err != nil {
		t.Fatalf("expected response metadata file: %v", err)
	}
	if _, err := os.Stat(respBodyPath); err != nil {
		t.Fatalf("expected response body file: %v", err)
	}

	fsReloaded, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage reload error: %v", err)
	}

	reloadedSpec, err := fsReloaded.GetSpec("spec-1")
	if err != nil {
		t.Fatalf("GetSpec error: %v", err)
	}
	if reloadedSpec.Content != specContent {
		t.Fatalf("expected spec content to reload")
	}

	reloadedCfg, err := fsReloaded.GetResponseConfig("resp-1")
	if err != nil {
		t.Fatalf("GetResponseConfig error: %v", err)
	}
	if reloadedCfg.Body != respBody {
		t.Fatalf("expected response body to reload")
	}
	if reloadedCfg.Tag != models.DefaultTagName {
		t.Fatalf("expected empty tag to default to %q, got %q", models.DefaultTagName, reloadedCfg.Tag)
	}

	if err := fsReloaded.DeleteResponseConfig("resp-1"); err != nil {
		t.Fatalf("DeleteResponseConfig error: %v", err)
	}
	if _, err := os.Stat(respMetaPath); !os.IsNotExist(err) {
		t.Fatalf("expected response metadata file to be removed")
	}
	if _, err := os.Stat(respBodyPath); !os.IsNotExist(err) {
		t.Fatalf("expected response body file to be removed")
	}

	if err := fsReloaded.DeleteSpec("spec-1"); err != nil {
		t.Fatalf("DeleteSpec error: %v", err)
	}
	if _, err := os.Stat(specMetaPath); !os.IsNotExist(err) {
		t.Fatalf("expected spec metadata file to be removed")
	}
	if _, err := os.Stat(specContentPath); !os.IsNotExist(err) {
		t.Fatalf("expected spec content file to be removed")
	}
}

func TestFileStorage_TagCRUD(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage error: %v", err)
	}

	tag := &models.Tag{
		Name:        "blue",
		Description: "primary",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := fs.CreateTag(tag); err != nil {
		t.Fatalf("CreateTag error: %v", err)
	}

	got, err := fs.GetTag("blue")
	if err != nil {
		t.Fatalf("GetTag error: %v", err)
	}
	if got.Description != "primary" {
		t.Fatalf("expected description to be saved")
	}

	updated := &models.Tag{
		Name:        "blue",
		Description: "updated",
		CreatedAt:   tag.CreatedAt,
		UpdatedAt:   time.Now(),
	}
	if err := fs.UpdateTag("blue", updated); err != nil {
		t.Fatalf("UpdateTag error: %v", err)
	}

	got, err = fs.GetTag("blue")
	if err != nil {
		t.Fatalf("GetTag error: %v", err)
	}
	if got.Description != "updated" {
		t.Fatalf("expected description to be updated")
	}

	if err := fs.DeleteTag("blue"); err != nil {
		t.Fatalf("DeleteTag error: %v", err)
	}
	if _, err := fs.GetTag("blue"); err == nil {
		t.Fatalf("expected tag to be deleted")
	}
}

func TestFileStorage_OperationsAndResponses(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage error: %v", err)
	}

	spec := &models.Spec{ID: "spec-1", Name: "Spec", Enabled: true}
	if err := fs.CreateSpec(spec); err != nil {
		t.Fatalf("CreateSpec error: %v", err)
	}

	spec.Name = "Spec Updated"
	if err := fs.UpdateSpec(spec); err != nil {
		t.Fatalf("UpdateSpec error: %v", err)
	}

	allSpecs, err := fs.GetAllSpecs()
	if err != nil || len(allSpecs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(allSpecs))
	}

	enabledSpecs, err := fs.GetEnabledSpecs()
	if err != nil || len(enabledSpecs) != 1 {
		t.Fatalf("expected 1 enabled spec, got %d", len(enabledSpecs))
	}

	spec.Enabled = false
	if err := fs.UpdateSpec(spec); err != nil {
		t.Fatalf("UpdateSpec error: %v", err)
	}
	enabledSpecs, err = fs.GetEnabledSpecs()
	if err != nil || len(enabledSpecs) != 0 {
		t.Fatalf("expected 0 enabled specs, got %d", len(enabledSpecs))
	}

	op := &models.Operation{ID: "op-1", SpecID: spec.ID, Method: "GET", Path: "/users"}
	if err := fs.CreateOperation(op); err != nil {
		t.Fatalf("CreateOperation error: %v", err)
	}

	if _, err := fs.GetOperation("op-1"); err != nil {
		t.Fatalf("GetOperation error: %v", err)
	}

	bySpec, err := fs.GetOperationsBySpec(spec.ID)
	if err != nil || len(bySpec) != 1 {
		t.Fatalf("expected 1 op by spec, got %d", len(bySpec))
	}

	allOps, err := fs.GetAllOperations()
	if err != nil || len(allOps) != 1 {
		t.Fatalf("expected 1 op, got %d", len(allOps))
	}

	op.Summary = "updated"
	if err := fs.UpdateOperation(op); err != nil {
		t.Fatalf("UpdateOperation error: %v", err)
	}

	if err := fs.DeleteOperation("op-1"); err != nil {
		t.Fatalf("DeleteOperation error: %v", err)
	}

	op2 := &models.Operation{ID: "op-2", SpecID: spec.ID, Method: "POST", Path: "/users"}
	op3 := &models.Operation{ID: "op-3", SpecID: "spec-2", Method: "GET", Path: "/items"}
	_ = fs.CreateOperation(op2)
	_ = fs.CreateOperation(op3)
	if err := fs.DeleteOperationsBySpec(spec.ID); err != nil {
		t.Fatalf("DeleteOperationsBySpec error: %v", err)
	}

	if _, err := fs.GetOperation("op-2"); err == nil {
		t.Fatalf("expected op-2 to be deleted")
	}
	if _, err := fs.GetOperation("op-3"); err != nil {
		t.Fatalf("expected op-3 to remain")
	}

	cfg1 := &models.ResponseConfig{ID: "resp-1", OperationID: "op-3", Name: "First", Priority: 1, StatusCode: 200}
	cfg2 := &models.ResponseConfig{ID: "resp-2", OperationID: "op-3", Name: "Second", Priority: 0, StatusCode: 200}
	if err := fs.CreateResponseConfig(cfg1); err != nil {
		t.Fatalf("CreateResponseConfig error: %v", err)
	}
	if err := fs.CreateResponseConfig(cfg2); err != nil {
		t.Fatalf("CreateResponseConfig error: %v", err)
	}

	configs, err := fs.GetResponseConfigsByOperation("op-3")
	if err != nil || len(configs) != 2 {
		t.Fatalf("expected 2 response configs, got %d", len(configs))
	}
	if configs[0].ID != "resp-2" {
		t.Fatalf("expected configs sorted by priority")
	}

	cfg2.Description = "updated"
	if err := fs.UpdateResponseConfig(cfg2); err != nil {
		t.Fatalf("UpdateResponseConfig error: %v", err)
	}

	if err := fs.DeleteResponseConfigsByOperation("op-3"); err != nil {
		t.Fatalf("DeleteResponseConfigsByOperation error: %v", err)
	}
	configs, _ = fs.GetResponseConfigsByOperation("op-3")
	if len(configs) != 0 {
		t.Fatalf("expected response configs to be deleted")
	}

	if err := fs.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

// TestFileStorage_OperationCustomization verifies SignatureConfig is persisted
// and reloaded correctly via UpdateOperation / loadOperationCustomization.
func TestFileStorage_OperationCustomization(t *testing.T) {
	baseDir := t.TempDir()

	specContent := `{"openapi":"3.0.0","info":{"title":"T","version":"1.0"},"paths":{"/items":{"get":{"operationId":"listItems","responses":{"200":{"description":"ok"}}}}}}`

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	p := parser.NewParser()
	parseResult, err := p.Parse(specContent, "/v1")
	if err != nil {
		t.Fatalf("parser.Parse: %v", err)
	}

	if err := fs.CreateSpec(parseResult.Spec); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	for _, op := range parseResult.Operations {
		if err := fs.CreateOperation(op); err != nil {
			t.Fatalf("CreateOperation: %v", err)
		}
	}

	ops, err := fs.GetOperationsBySpec(parseResult.Spec.ID)
	if err != nil || len(ops) == 0 {
		t.Fatalf("expected operations parsed from spec, got %d (%v)", len(ops), err)
	}
	op := ops[0]

	// Attach a SignatureConfig
	op.SignatureConfig = &models.SignatureConfig{
		PathParams:  []string{"id"},
		QueryParams: []string{"page"},
		Headers:     []string{"X-Tenant"},
		IncludeBody: false,
	}

	if err := fs.UpdateOperation(op); err != nil {
		t.Fatalf("UpdateOperation: %v", err)
	}

	// Reload from disk — SignatureConfig should be preserved
	fsReloaded, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage reload: %v", err)
	}

	reloadedOps, _ := fsReloaded.GetOperationsBySpec(parseResult.Spec.ID)
	if len(reloadedOps) == 0 {
		t.Fatal("expected operations after reload")
	}
	ro := reloadedOps[0]

	if ro.SignatureConfig == nil {
		t.Fatal("expected SignatureConfig to survive reload")
	}
	if len(ro.SignatureConfig.PathParams) != 1 || ro.SignatureConfig.PathParams[0] != "id" {
		t.Errorf("unexpected PathParams after reload: %v", ro.SignatureConfig.PathParams)
	}
	if ro.SignatureConfig.IncludeBody {
		t.Error("expected IncludeBody=false after reload")
	}
	if len(ro.SignatureConfig.Headers) != 1 || ro.SignatureConfig.Headers[0] != "X-Tenant" {
		t.Errorf("unexpected Headers after reload: %v", ro.SignatureConfig.Headers)
	}
}

// TestFileStorage_SpecBackendURIAndProxyMode verifies BackendURI and ProxyMode
// are saved to and reloaded from disk.
func TestFileStorage_SpecBackendURIAndProxyMode(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	spec := &models.Spec{
		ID:         "spec-be",
		Name:       "Backend Test",
		BackendURI: "http://real-backend:9090",
		ProxyMode:  true,
		Enabled:    true,
	}
	if err := fs.CreateSpec(spec); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	fsReloaded, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}

	reloaded, err := fsReloaded.GetSpec("spec-be")
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if reloaded.BackendURI != "http://real-backend:9090" {
		t.Errorf("expected BackendURI, got %q", reloaded.BackendURI)
	}
	// ProxyMode is reset on reload (same rule as Tracing)
	// Actually ProxyMode is NOT reset — only Tracing is. Verify it persists.
	if !reloaded.ProxyMode {
		t.Error("expected ProxyMode to survive reload")
	}
}

// TestFileStorage_LoadOperationCustomization_MissingFile verifies that loading
// a customization for an operation that has no file returns nil without error.
func TestFileStorage_LoadOperationCustomization_MissingFile(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	custom, err := fs.loadOperationCustomization("nonexistent-op-id")
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
	if custom != nil {
		t.Errorf("expected nil customization for missing file, got: %+v", custom)
	}
}

// TestFileStorage_UpdateResponseConfig_Persists verifies UpdateResponseConfig
// writes new body to disk and reloads correctly.
func TestFileStorage_UpdateResponseConfig_Persists(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	cfg := &models.ResponseConfig{
		ID:          "resp-upd",
		OperationID: "op-upd",
		Name:        "Initial",
		StatusCode:  200,
		Body:        `{"v":1}`,
		Enabled:     true,
	}
	if err := fs.CreateResponseConfig(cfg); err != nil {
		t.Fatalf("CreateResponseConfig: %v", err)
	}

	cfg.Body = `{"v":2}`
	cfg.Name = "Updated"
	if err := fs.UpdateResponseConfig(cfg); err != nil {
		t.Fatalf("UpdateResponseConfig: %v", err)
	}

	fsReloaded, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	reloaded, err := fsReloaded.GetResponseConfig("resp-upd")
	if err != nil {
		t.Fatalf("GetResponseConfig: %v", err)
	}
	if reloaded.Body != `{"v":2}` {
		t.Errorf("expected updated body, got %q", reloaded.Body)
	}
	if reloaded.Name != "Updated" {
		t.Errorf("expected updated name, got %q", reloaded.Name)
	}
}

// ---- FileStorage script tests ----

func TestFileStorage_ScriptCRUD(t *testing.T) {
	baseDir := t.TempDir()
	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	script := &models.Script{
		ID:      "s1",
		Name:    "My Script",
		Source:  "def run(req): return 1",
		Timeout: 100,
		Enabled: true,
	}

	// Create
	if err := fs.CreateScript(script); err != nil {
		t.Fatalf("CreateScript: %v", err)
	}

	// Get
	got, err := fs.GetScript("s1")
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if got.Name != "My Script" {
		t.Errorf("Name: got %q", got.Name)
	}
	if got.Source != "def run(req): return 1" {
		t.Errorf("Source: got %q", got.Source)
	}

	// Duplicate should fail
	if err := fs.CreateScript(script); err == nil {
		t.Error("Expected error on duplicate CreateScript")
	}

	// Update
	script.Name = "Updated Script"
	script.Source = "def run(req): return 2"
	if err := fs.UpdateScript(script); err != nil {
		t.Fatalf("UpdateScript: %v", err)
	}

	// GetAll
	all, err := fs.GetAllScripts()
	if err != nil {
		t.Fatalf("GetAllScripts: %v", err)
	}
	if len(all) != 1 || all[0].Name != "Updated Script" {
		t.Errorf("GetAllScripts: got %+v", all)
	}

	// Delete
	if err := fs.DeleteScript("s1"); err != nil {
		t.Fatalf("DeleteScript: %v", err)
	}
	if _, err := fs.GetScript("s1"); err == nil {
		t.Error("Expected error after delete")
	}
}

func TestFileStorage_ScriptPersistsAcrossReload(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	script := &models.Script{
		ID:      "persist-s1",
		Name:    "Persistent",
		Source:  `def run(req): return {"ok": True}`,
		Timeout: 50,
		Enabled: true,
	}
	if err := fs.CreateScript(script); err != nil {
		t.Fatalf("CreateScript: %v", err)
	}

	// Reload from disk
	fs2, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	got, err := fs2.GetScript("persist-s1")
	if err != nil {
		t.Fatalf("GetScript after reload: %v", err)
	}
	if got.Name != "Persistent" {
		t.Errorf("Name after reload: got %q", got.Name)
	}
	if got.Source != `def run(req): return {"ok": True}` {
		t.Errorf("Source after reload: got %q", got.Source)
	}
}

func TestFileStorage_ScriptBindingCRUD(t *testing.T) {
	baseDir := t.TempDir()
	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	b1 := &models.ScriptBinding{ID: "b1", OperationID: "op-1", ScriptID: "s1", OutputKey: "result", Order: 0, Enabled: true}
	b2 := &models.ScriptBinding{ID: "b2", OperationID: "op-1", ScriptID: "s2", OutputKey: "extra", Order: 1, Enabled: true}

	if err := fs.CreateScriptBinding(b1); err != nil {
		t.Fatalf("CreateScriptBinding b1: %v", err)
	}
	if err := fs.CreateScriptBinding(b2); err != nil {
		t.Fatalf("CreateScriptBinding b2: %v", err)
	}

	// Duplicate
	if err := fs.CreateScriptBinding(b1); err == nil {
		t.Error("Expected error on duplicate binding")
	}

	// GetScriptBindings
	bindings, err := fs.GetScriptBindings("op-1")
	if err != nil {
		t.Fatalf("GetScriptBindings: %v", err)
	}
	if len(bindings) != 2 {
		t.Fatalf("Expected 2 bindings, got %d", len(bindings))
	}

	// UpdateScriptBinding
	b1.OutputKey = "updated"
	if err := fs.UpdateScriptBinding(b1); err != nil {
		t.Fatalf("UpdateScriptBinding: %v", err)
	}
	bindings, _ = fs.GetScriptBindings("op-1")
	found := false
	for _, b := range bindings {
		if b.ID == "b1" && b.OutputKey == "updated" {
			found = true
		}
	}
	if !found {
		t.Error("Expected binding b1 to have outputKey 'updated'")
	}

	// DeleteScriptBinding
	if err := fs.DeleteScriptBinding("b1"); err != nil {
		t.Fatalf("DeleteScriptBinding: %v", err)
	}
	bindings, _ = fs.GetScriptBindings("op-1")
	if len(bindings) != 1 {
		t.Errorf("Expected 1 binding after delete, got %d", len(bindings))
	}

	// DeleteScriptBinding on non-existent
	if err := fs.DeleteScriptBinding("nonexistent"); err == nil {
		t.Error("Expected error deleting non-existent binding")
	}
}

func TestFileStorage_DeleteScriptBindingsByScript(t *testing.T) {
	baseDir := t.TempDir()
	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	_ = fs.CreateScriptBinding(&models.ScriptBinding{ID: "b1", OperationID: "op-1", ScriptID: "s1", OutputKey: "a"})
	_ = fs.CreateScriptBinding(&models.ScriptBinding{ID: "b2", OperationID: "op-2", ScriptID: "s1", OutputKey: "b"})
	_ = fs.CreateScriptBinding(&models.ScriptBinding{ID: "b3", OperationID: "op-3", ScriptID: "s2", OutputKey: "c"})

	if err := fs.DeleteScriptBindingsByScript("s1"); err != nil {
		t.Fatalf("DeleteScriptBindingsByScript: %v", err)
	}

	b1, _ := fs.GetScriptBindings("op-1")
	b2, _ := fs.GetScriptBindings("op-2")
	b3, _ := fs.GetScriptBindings("op-3")

	if len(b1) != 0 || len(b2) != 0 {
		t.Errorf("Expected s1 bindings deleted, got %d and %d", len(b1), len(b2))
	}
	if len(b3) != 1 {
		t.Errorf("Expected s2 binding retained, got %d", len(b3))
	}
}

func TestFileStorage_ScriptBindingPersistsAcrossReload(t *testing.T) {
	baseDir := t.TempDir()

	fs, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}

	_ = fs.CreateScriptBinding(&models.ScriptBinding{
		ID: "b1", OperationID: "op-1", ScriptID: "s1", OutputKey: "result", Order: 0, Enabled: true,
	})

	fs2, err := NewFileStorage(baseDir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	bindings, err := fs2.GetScriptBindings("op-1")
	if err != nil {
		t.Fatalf("GetScriptBindings after reload: %v", err)
	}
	if len(bindings) != 1 || bindings[0].OutputKey != "result" {
		t.Errorf("Expected 1 binding with outputKey 'result' after reload, got %+v", bindings)
	}
}
