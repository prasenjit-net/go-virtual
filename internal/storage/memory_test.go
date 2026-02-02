package storage

import (
	"sync"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestNewMemoryStorage(t *testing.T) {
	s := NewMemoryStorage()
	if s == nil {
		t.Fatal("NewMemoryStorage returned nil")
	}
	if s.specs == nil || s.operations == nil || s.responseConfigs == nil {
		t.Fatal("Storage maps not initialized")
	}
}

// Spec tests
func TestCreateSpec(t *testing.T) {
	s := NewMemoryStorage()

	spec := &models.Spec{
		ID:        "spec-1",
		Name:      "Test Spec",
		Version:   "1.0.0",
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := s.CreateSpec(spec)
	if err != nil {
		t.Fatalf("CreateSpec failed: %v", err)
	}

	// Try to create duplicate
	err = s.CreateSpec(spec)
	if err == nil {
		t.Error("Expected error when creating duplicate spec")
	}
}

func TestGetSpec(t *testing.T) {
	s := NewMemoryStorage()

	spec := &models.Spec{
		ID:      "spec-1",
		Name:    "Test Spec",
		Version: "1.0.0",
	}
	_ = s.CreateSpec(spec)

	// Get existing spec
	result, err := s.GetSpec("spec-1")
	if err != nil {
		t.Fatalf("GetSpec failed: %v", err)
	}
	if result.Name != "Test Spec" {
		t.Errorf("Expected name 'Test Spec', got %q", result.Name)
	}

	// Get non-existent spec
	_, err = s.GetSpec("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent spec")
	}
}

func TestGetAllSpecs(t *testing.T) {
	s := NewMemoryStorage()

	// Empty storage
	specs, err := s.GetAllSpecs()
	if err != nil {
		t.Fatalf("GetAllSpecs failed: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("Expected 0 specs, got %d", len(specs))
	}

	// Add specs
	_ = s.CreateSpec(&models.Spec{ID: "spec-1", Name: "B Spec"})
	_ = s.CreateSpec(&models.Spec{ID: "spec-2", Name: "A Spec"})
	_ = s.CreateSpec(&models.Spec{ID: "spec-3", Name: "C Spec"})

	specs, err = s.GetAllSpecs()
	if err != nil {
		t.Fatalf("GetAllSpecs failed: %v", err)
	}
	if len(specs) != 3 {
		t.Errorf("Expected 3 specs, got %d", len(specs))
	}

	// Check sorted by name
	if specs[0].Name != "A Spec" {
		t.Errorf("Expected first spec to be 'A Spec', got %q", specs[0].Name)
	}
}

func TestGetEnabledSpecs(t *testing.T) {
	s := NewMemoryStorage()

	_ = s.CreateSpec(&models.Spec{ID: "spec-1", Name: "Enabled", Enabled: true})
	_ = s.CreateSpec(&models.Spec{ID: "spec-2", Name: "Disabled", Enabled: false})
	_ = s.CreateSpec(&models.Spec{ID: "spec-3", Name: "Enabled2", Enabled: true})

	specs, err := s.GetEnabledSpecs()
	if err != nil {
		t.Fatalf("GetEnabledSpecs failed: %v", err)
	}
	if len(specs) != 2 {
		t.Errorf("Expected 2 enabled specs, got %d", len(specs))
	}

	for _, spec := range specs {
		if !spec.Enabled {
			t.Error("Got disabled spec in enabled list")
		}
	}
}

func TestUpdateSpec(t *testing.T) {
	s := NewMemoryStorage()

	spec := &models.Spec{ID: "spec-1", Name: "Original"}
	_ = s.CreateSpec(spec)

	// Update existing
	spec.Name = "Updated"
	err := s.UpdateSpec(spec)
	if err != nil {
		t.Fatalf("UpdateSpec failed: %v", err)
	}

	result, _ := s.GetSpec("spec-1")
	if result.Name != "Updated" {
		t.Errorf("Expected name 'Updated', got %q", result.Name)
	}

	// Update non-existent
	err = s.UpdateSpec(&models.Spec{ID: "nonexistent"})
	if err == nil {
		t.Error("Expected error when updating non-existent spec")
	}
}

func TestDeleteSpec(t *testing.T) {
	s := NewMemoryStorage()

	spec := &models.Spec{ID: "spec-1", Name: "Test"}
	_ = s.CreateSpec(spec)

	// Delete existing
	err := s.DeleteSpec("spec-1")
	if err != nil {
		t.Fatalf("DeleteSpec failed: %v", err)
	}

	_, err = s.GetSpec("spec-1")
	if err == nil {
		t.Error("Spec should be deleted")
	}

	// Delete non-existent
	err = s.DeleteSpec("nonexistent")
	if err == nil {
		t.Error("Expected error when deleting non-existent spec")
	}
}

// Operation tests
func TestCreateOperation(t *testing.T) {
	s := NewMemoryStorage()

	op := &models.Operation{
		ID:     "op-1",
		SpecID: "spec-1",
		Method: "GET",
		Path:   "/users",
	}

	err := s.CreateOperation(op)
	if err != nil {
		t.Fatalf("CreateOperation failed: %v", err)
	}

	// Try to create duplicate
	err = s.CreateOperation(op)
	if err == nil {
		t.Error("Expected error when creating duplicate operation")
	}
}

func TestGetOperation(t *testing.T) {
	s := NewMemoryStorage()

	op := &models.Operation{
		ID:     "op-1",
		SpecID: "spec-1",
		Method: "GET",
		Path:   "/users",
	}
	_ = s.CreateOperation(op)

	// Get existing
	result, err := s.GetOperation("op-1")
	if err != nil {
		t.Fatalf("GetOperation failed: %v", err)
	}
	if result.Method != "GET" {
		t.Errorf("Expected method 'GET', got %q", result.Method)
	}

	// Get non-existent
	_, err = s.GetOperation("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent operation")
	}
}

func TestGetOperationsBySpec(t *testing.T) {
	s := NewMemoryStorage()

	_ = s.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET", Path: "/users"})
	_ = s.CreateOperation(&models.Operation{ID: "op-2", SpecID: "spec-1", Method: "POST", Path: "/users"})
	_ = s.CreateOperation(&models.Operation{ID: "op-3", SpecID: "spec-2", Method: "GET", Path: "/items"})
	_ = s.CreateOperation(&models.Operation{ID: "op-4", SpecID: "spec-1", Method: "GET", Path: "/items"})

	ops, err := s.GetOperationsBySpec("spec-1")
	if err != nil {
		t.Fatalf("GetOperationsBySpec failed: %v", err)
	}
	if len(ops) != 3 {
		t.Errorf("Expected 3 operations, got %d", len(ops))
	}

	// Check sorted by path, then method
	if ops[0].Path != "/items" {
		t.Errorf("Expected first path '/items', got %q", ops[0].Path)
	}
	if ops[1].Path != "/users" {
		t.Errorf("Expected second path '/users', got %q", ops[1].Path)
	}
	if ops[1].Method != "GET" {
		t.Errorf("Expected second method 'GET', got %q", ops[1].Method)
	}
	if ops[2].Method != "POST" {
		t.Errorf("Expected third method 'POST', got %q", ops[2].Method)
	}
}

func TestGetAllOperations(t *testing.T) {
	s := NewMemoryStorage()

	_ = s.CreateOperation(&models.Operation{ID: "op-1", SpecID: "spec-1"})
	_ = s.CreateOperation(&models.Operation{ID: "op-2", SpecID: "spec-2"})

	ops, err := s.GetAllOperations()
	if err != nil {
		t.Fatalf("GetAllOperations failed: %v", err)
	}
	if len(ops) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(ops))
	}
}

func TestUpdateOperation(t *testing.T) {
	s := NewMemoryStorage()

	op := &models.Operation{ID: "op-1", SpecID: "spec-1", Method: "GET"}
	_ = s.CreateOperation(op)

	// Update existing
	op.Method = "POST"
	err := s.UpdateOperation(op)
	if err != nil {
		t.Fatalf("UpdateOperation failed: %v", err)
	}

	result, _ := s.GetOperation("op-1")
	if result.Method != "POST" {
		t.Errorf("Expected method 'POST', got %q", result.Method)
	}

	// Update non-existent
	err = s.UpdateOperation(&models.Operation{ID: "nonexistent"})
	if err == nil {
		t.Error("Expected error when updating non-existent operation")
	}
}

func TestDeleteOperation(t *testing.T) {
	s := NewMemoryStorage()

	op := &models.Operation{ID: "op-1", SpecID: "spec-1"}
	_ = s.CreateOperation(op)

	// Delete existing
	err := s.DeleteOperation("op-1")
	if err != nil {
		t.Fatalf("DeleteOperation failed: %v", err)
	}

	_, err = s.GetOperation("op-1")
	if err == nil {
		t.Error("Operation should be deleted")
	}

	// Delete non-existent
	err = s.DeleteOperation("nonexistent")
	if err == nil {
		t.Error("Expected error when deleting non-existent operation")
	}
}

// ResponseConfig tests
func TestCreateResponseConfig(t *testing.T) {
	s := NewMemoryStorage()

	rc := &models.ResponseConfig{
		ID:          "rc-1",
		OperationID: "op-1",
		Name:        "Default Response",
		StatusCode:  200,
	}

	err := s.CreateResponseConfig(rc)
	if err != nil {
		t.Fatalf("CreateResponseConfig failed: %v", err)
	}

	// Try to create duplicate
	err = s.CreateResponseConfig(rc)
	if err == nil {
		t.Error("Expected error when creating duplicate response config")
	}
}

func TestGetResponseConfig(t *testing.T) {
	s := NewMemoryStorage()

	rc := &models.ResponseConfig{
		ID:          "rc-1",
		OperationID: "op-1",
		Name:        "Default",
		StatusCode:  200,
	}
	_ = s.CreateResponseConfig(rc)

	// Get existing
	result, err := s.GetResponseConfig("rc-1")
	if err != nil {
		t.Fatalf("GetResponseConfig failed: %v", err)
	}
	if result.Name != "Default" {
		t.Errorf("Expected name 'Default', got %q", result.Name)
	}

	// Get non-existent
	_, err = s.GetResponseConfig("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent response config")
	}
}

func TestGetResponseConfigsByOperation(t *testing.T) {
	s := NewMemoryStorage()

	_ = s.CreateResponseConfig(&models.ResponseConfig{ID: "rc-1", OperationID: "op-1", Priority: 2})
	_ = s.CreateResponseConfig(&models.ResponseConfig{ID: "rc-2", OperationID: "op-1", Priority: 1})
	_ = s.CreateResponseConfig(&models.ResponseConfig{ID: "rc-3", OperationID: "op-2", Priority: 0})
	_ = s.CreateResponseConfig(&models.ResponseConfig{ID: "rc-4", OperationID: "op-1", Priority: 0})

	rcs, err := s.GetResponseConfigsByOperation("op-1")
	if err != nil {
		t.Fatalf("GetResponseConfigsByOperation failed: %v", err)
	}
	if len(rcs) != 3 {
		t.Errorf("Expected 3 response configs, got %d", len(rcs))
	}

	// Check sorted by priority
	if rcs[0].Priority != 0 {
		t.Errorf("Expected first priority 0, got %d", rcs[0].Priority)
	}
	if rcs[1].Priority != 1 {
		t.Errorf("Expected second priority 1, got %d", rcs[1].Priority)
	}
	if rcs[2].Priority != 2 {
		t.Errorf("Expected third priority 2, got %d", rcs[2].Priority)
	}
}

func TestUpdateResponseConfig(t *testing.T) {
	s := NewMemoryStorage()

	rc := &models.ResponseConfig{ID: "rc-1", OperationID: "op-1", StatusCode: 200}
	_ = s.CreateResponseConfig(rc)

	// Update existing
	rc.StatusCode = 201
	err := s.UpdateResponseConfig(rc)
	if err != nil {
		t.Fatalf("UpdateResponseConfig failed: %v", err)
	}

	result, _ := s.GetResponseConfig("rc-1")
	if result.StatusCode != 201 {
		t.Errorf("Expected status code 201, got %d", result.StatusCode)
	}

	// Update non-existent
	err = s.UpdateResponseConfig(&models.ResponseConfig{ID: "nonexistent"})
	if err == nil {
		t.Error("Expected error when updating non-existent response config")
	}
}

func TestDeleteResponseConfig(t *testing.T) {
	s := NewMemoryStorage()

	rc := &models.ResponseConfig{ID: "rc-1", OperationID: "op-1"}
	_ = s.CreateResponseConfig(rc)

	// Delete existing
	err := s.DeleteResponseConfig("rc-1")
	if err != nil {
		t.Fatalf("DeleteResponseConfig failed: %v", err)
	}

	_, err = s.GetResponseConfig("rc-1")
	if err == nil {
		t.Error("Response config should be deleted")
	}

	// Delete non-existent
	err = s.DeleteResponseConfig("nonexistent")
	if err == nil {
		t.Error("Expected error when deleting non-existent response config")
	}
}

func TestDeleteResponseConfigsByOperation(t *testing.T) {
	s := NewMemoryStorage()

	_ = s.CreateResponseConfig(&models.ResponseConfig{ID: "rc-1", OperationID: "op-1"})
	_ = s.CreateResponseConfig(&models.ResponseConfig{ID: "rc-2", OperationID: "op-1"})
	_ = s.CreateResponseConfig(&models.ResponseConfig{ID: "rc-3", OperationID: "op-2"})

	err := s.DeleteResponseConfigsByOperation("op-1")
	if err != nil {
		t.Fatalf("DeleteResponseConfigsByOperation failed: %v", err)
	}

	rcs, _ := s.GetResponseConfigsByOperation("op-1")
	if len(rcs) != 0 {
		t.Errorf("Expected 0 response configs for op-1, got %d", len(rcs))
	}

	// op-2 should still have its response config
	rcs, _ = s.GetResponseConfigsByOperation("op-2")
	if len(rcs) != 1 {
		t.Errorf("Expected 1 response config for op-2, got %d", len(rcs))
	}
}

// Concurrency tests
func TestConcurrentSpecAccess(t *testing.T) {
	s := NewMemoryStorage()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent creates
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			spec := &models.Spec{
				ID:   "spec-" + string(rune('a'+n%26)) + string(rune('0'+n/26)),
				Name: "Test Spec",
			}
			_ = s.CreateSpec(spec)
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.GetAllSpecs()
		}()
	}
	wg.Wait()
}

func TestConcurrentOperationAccess(t *testing.T) {
	s := NewMemoryStorage()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent creates
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			op := &models.Operation{
				ID:     "op-" + string(rune('a'+n%26)) + string(rune('0'+n/26)),
				SpecID: "spec-1",
			}
			_ = s.CreateOperation(op)
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.GetOperationsBySpec("spec-1")
		}()
	}
	wg.Wait()
}

func TestConcurrentResponseConfigAccess(t *testing.T) {
	s := NewMemoryStorage()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent creates
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			rc := &models.ResponseConfig{
				ID:          "rc-" + string(rune('a'+n%26)) + string(rune('0'+n/26)),
				OperationID: "op-1",
			}
			_ = s.CreateResponseConfig(rc)
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.GetResponseConfigsByOperation("op-1")
		}()
	}
	wg.Wait()
}
