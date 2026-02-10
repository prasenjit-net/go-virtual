package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
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
