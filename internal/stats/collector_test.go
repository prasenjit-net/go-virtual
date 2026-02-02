package stats

import (
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("NewCollector returned nil")
	}
	if c.operations == nil {
		t.Fatal("Operations map not initialized")
	}
	if c.recentErrors == nil {
		t.Fatal("Recent errors slice not initialized")
	}
	if c.hourlyStats == nil {
		t.Fatal("Hourly stats map not initialized")
	}
	if c.maxErrors != 100 {
		t.Errorf("Expected maxErrors 100, got %d", c.maxErrors)
	}
	if c.maxHourlySlots != 168 {
		t.Errorf("Expected maxHourlySlots 168, got %d", c.maxHourlySlots)
	}
}

func TestRecordRequest(t *testing.T) {
	c := NewCollector()

	// Record first request
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)

	stats := c.GetGlobalStats(1, 1)
	if stats.TotalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", stats.TotalRequests)
	}
	if stats.TotalErrors != 0 {
		t.Errorf("Expected 0 errors, got %d", stats.TotalErrors)
	}

	// Record error request
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 50*time.Millisecond, true)

	stats = c.GetGlobalStats(1, 1)
	if stats.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", stats.TotalRequests)
	}
	if stats.TotalErrors != 1 {
		t.Errorf("Expected 1 error, got %d", stats.TotalErrors)
	}
}

func TestRecordRequest_MultipleOperations(t *testing.T) {
	c := NewCollector()

	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)
	c.RecordRequest("spec-1", "op-2", "POST", "/users", 200*time.Millisecond, false)
	c.RecordRequest("spec-2", "op-3", "GET", "/items", 150*time.Millisecond, false)

	stats := c.GetGlobalStats(2, 3)
	if stats.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", stats.TotalRequests)
	}
	if stats.ActiveSpecs != 2 {
		t.Errorf("Expected 2 active specs, got %d", stats.ActiveSpecs)
	}
	if stats.TotalOperations != 3 {
		t.Errorf("Expected 3 total operations, got %d", stats.TotalOperations)
	}
}

func TestRecordRequest_MinMaxTime(t *testing.T) {
	c := NewCollector()

	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 50*time.Millisecond, false)
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 200*time.Millisecond, false)

	opStats := c.GetOperationStats("op-1")
	if opStats == nil {
		t.Fatal("Expected operation stats")
	}
	
	// Min should be 50ms
	if opStats.MinResponseTimeMs != 50.0 {
		t.Errorf("Expected min time 50ms, got %v", opStats.MinResponseTimeMs)
	}
	
	// Max should be 200ms
	if opStats.MaxResponseTimeMs != 200.0 {
		t.Errorf("Expected max time 200ms, got %v", opStats.MaxResponseTimeMs)
	}
}

func TestRecordError(t *testing.T) {
	c := NewCollector()

	c.RecordError("spec-1", "op-1", "/users", "GET", 500, "Internal Server Error")

	stats := c.GetGlobalStats(1, 1)
	if len(stats.RecentErrors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(stats.RecentErrors))
	}
	if stats.RecentErrors[0].StatusCode != 500 {
		t.Errorf("Expected status code 500, got %d", stats.RecentErrors[0].StatusCode)
	}
	if stats.RecentErrors[0].Error != "Internal Server Error" {
		t.Errorf("Expected error message 'Internal Server Error', got %q", stats.RecentErrors[0].Error)
	}
}

func TestRecordError_MaxLimit(t *testing.T) {
	c := NewCollector()
	c.maxErrors = 5

	// Record more than max errors
	for i := 0; i < 10; i++ {
		c.RecordError("spec-1", "op-1", "/users", "GET", 500, "Error")
	}

	stats := c.GetGlobalStats(1, 1)
	if len(stats.RecentErrors) != 5 {
		t.Errorf("Expected 5 errors (max), got %d", len(stats.RecentErrors))
	}
}

func TestGetGlobalStats(t *testing.T) {
	c := NewCollector()

	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 200*time.Millisecond, false)
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 50*time.Millisecond, true)

	stats := c.GetGlobalStats(5, 10)

	if stats.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", stats.TotalRequests)
	}
	if stats.TotalErrors != 1 {
		t.Errorf("Expected 1 error, got %d", stats.TotalErrors)
	}
	if stats.ActiveSpecs != 5 {
		t.Errorf("Expected 5 active specs, got %d", stats.ActiveSpecs)
	}
	if stats.TotalOperations != 10 {
		t.Errorf("Expected 10 total operations, got %d", stats.TotalOperations)
	}
	if stats.Uptime == "" {
		t.Error("Expected non-empty uptime")
	}
}

func TestGetGlobalStats_TopOperations(t *testing.T) {
	c := NewCollector()

	// Create 15 operations with different request counts
	for i := 0; i < 15; i++ {
		opID := string(rune('a' + i))
		for j := 0; j <= i; j++ {
			c.RecordRequest("spec-1", opID, "GET", "/"+opID, 100*time.Millisecond, false)
		}
	}

	stats := c.GetGlobalStats(1, 15)

	// Should have at most 10 top operations
	if len(stats.TopOperations) > 10 {
		t.Errorf("Expected at most 10 top operations, got %d", len(stats.TopOperations))
	}

	// Should be sorted by total requests descending
	for i := 1; i < len(stats.TopOperations); i++ {
		if stats.TopOperations[i-1].TotalRequests < stats.TopOperations[i].TotalRequests {
			t.Error("Top operations should be sorted by total requests descending")
		}
	}
}

func TestGetGlobalStats_HourlyStats(t *testing.T) {
	c := NewCollector()

	// Record some requests
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, true)

	stats := c.GetGlobalStats(1, 1)
	
	if len(stats.RequestsByHour) != 24 {
		t.Errorf("Expected 24 hourly stats, got %d", len(stats.RequestsByHour))
	}

	// Current hour should have stats
	found := false
	for _, stat := range stats.RequestsByHour {
		if stat.Requests > 0 {
			found = true
			if stat.Requests != 2 {
				t.Errorf("Expected 2 requests, got %d", stat.Requests)
			}
			if stat.Errors != 1 {
				t.Errorf("Expected 1 error, got %d", stat.Errors)
			}
		}
	}
	if !found {
		t.Error("Expected to find hourly stats with requests")
	}
}

func TestGetOperationStats(t *testing.T) {
	c := NewCollector()

	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 200*time.Millisecond, true)

	stats := c.GetOperationStats("op-1")
	if stats == nil {
		t.Fatal("Expected operation stats")
	}
	if stats.TotalRequests != 2 {
		t.Errorf("Expected 2 requests, got %d", stats.TotalRequests)
	}
	if stats.TotalErrors != 1 {
		t.Errorf("Expected 1 error, got %d", stats.TotalErrors)
	}
	if stats.Method != "GET" {
		t.Errorf("Expected method 'GET', got %q", stats.Method)
	}
	if stats.Path != "/users" {
		t.Errorf("Expected path '/users', got %q", stats.Path)
	}

	// Non-existent operation
	stats = c.GetOperationStats("nonexistent")
	if stats != nil {
		t.Error("Expected nil for non-existent operation")
	}
}

func TestGetSpecStats(t *testing.T) {
	c := NewCollector()

	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)
	c.RecordRequest("spec-1", "op-2", "POST", "/users", 200*time.Millisecond, false)
	c.RecordRequest("spec-2", "op-3", "GET", "/items", 150*time.Millisecond, true)

	stats := c.GetSpecStats("spec-1", "Test Spec")

	if stats.SpecID != "spec-1" {
		t.Errorf("Expected spec ID 'spec-1', got %q", stats.SpecID)
	}
	if stats.SpecName != "Test Spec" {
		t.Errorf("Expected spec name 'Test Spec', got %q", stats.SpecName)
	}
	if stats.TotalRequests != 2 {
		t.Errorf("Expected 2 requests, got %d", stats.TotalRequests)
	}
	if stats.TotalErrors != 0 {
		t.Errorf("Expected 0 errors, got %d", stats.TotalErrors)
	}
	if len(stats.Operations) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(stats.Operations))
	}
}

func TestReset(t *testing.T) {
	c := NewCollector()

	// Add some data
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)
	c.RecordError("spec-1", "op-1", "/users", "GET", 500, "Error")

	// Verify data exists
	stats := c.GetGlobalStats(1, 1)
	if stats.TotalRequests != 1 {
		t.Error("Expected data before reset")
	}

	// Reset
	c.Reset()

	// Verify data cleared
	stats = c.GetGlobalStats(0, 0)
	if stats.TotalRequests != 0 {
		t.Errorf("Expected 0 requests after reset, got %d", stats.TotalRequests)
	}
	
	if len(stats.RecentErrors) != 0 {
		t.Errorf("Expected 0 errors after reset, got %d", len(stats.RecentErrors))
	}
}

func TestHourlyStatsCleanup(t *testing.T) {
	c := NewCollector()
	c.maxHourlySlots = 3

	// Manually add old hourly stats
	c.mu.Lock()
	c.hourlyStats["2024-01-01-00"] = &hourlyCounter{Hour: "2024-01-01-00", Requests: 1}
	c.hourlyStats["2024-01-01-01"] = &hourlyCounter{Hour: "2024-01-01-01", Requests: 1}
	c.hourlyStats["2024-01-01-02"] = &hourlyCounter{Hour: "2024-01-01-02", Requests: 1}
	c.hourlyStats["2024-01-01-03"] = &hourlyCounter{Hour: "2024-01-01-03", Requests: 1}
	c.hourlyStats["2024-01-01-04"] = &hourlyCounter{Hour: "2024-01-01-04", Requests: 1}
	c.mu.Unlock()

	// Trigger cleanup
	c.RecordRequest("spec-1", "op-1", "GET", "/users", 100*time.Millisecond, false)

	c.mu.RLock()
	count := len(c.hourlyStats)
	c.mu.RUnlock()

	// Should have max + 1 (new entry) = 4 or be cleaned up to max
	if count > c.maxHourlySlots+1 {
		t.Errorf("Expected at most %d hourly slots, got %d", c.maxHourlySlots+1, count)
	}
}

func TestConcurrentStatsAccess(t *testing.T) {
	c := NewCollector()

	done := make(chan bool)

	// Concurrent writers
	go func() {
		for i := 0; i < 100; i++ {
			c.RecordRequest("spec-1", "op-1", "GET", "/users", time.Duration(i)*time.Millisecond, i%5 == 0)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			c.RecordError("spec-1", "op-1", "/users", "GET", 500, "Error")
		}
		done <- true
	}()

	// Concurrent readers
	go func() {
		for i := 0; i < 100; i++ {
			_ = c.GetGlobalStats(1, 1)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = c.GetOperationStats("op-1")
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}

	// Verify data integrity
	stats := c.GetGlobalStats(1, 1)
	if stats.TotalRequests != 100 {
		t.Errorf("Expected 100 requests, got %d", stats.TotalRequests)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"seconds", 30 * time.Second},
		{"minutes", 5 * time.Minute},
		{"hours", 2 * time.Hour},
		{"days", 48 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result == "" {
				t.Error("Expected non-empty formatted duration")
			}
		})
	}
}
