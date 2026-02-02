package models

import (
	"testing"
	"time"
)

func TestAtomicOperationStat_ToOperationStat(t *testing.T) {
	aos := &AtomicOperationStat{
		OperationID: "op-1",
		SpecID:      "spec-1",
		Method:      "GET",
		Path:        "/users",
	}

	// Set values
	aos.TotalRequests.Store(100)
	aos.TotalErrors.Store(5)
	aos.TotalTimeNs.Store(1000000000) // 1 second = 1000ms
	aos.MinTimeNs.Store(5000000)      // 5ms
	aos.MaxTimeNs.Store(50000000)     // 50ms
	aos.LastRequestTime.Store(time.Now())

	stat := aos.ToOperationStat()

	if stat.OperationID != "op-1" {
		t.Errorf("Expected operation ID 'op-1', got %q", stat.OperationID)
	}
	if stat.SpecID != "spec-1" {
		t.Errorf("Expected spec ID 'spec-1', got %q", stat.SpecID)
	}
	if stat.Method != "GET" {
		t.Errorf("Expected method 'GET', got %q", stat.Method)
	}
	if stat.Path != "/users" {
		t.Errorf("Expected path '/users', got %q", stat.Path)
	}
	if stat.TotalRequests != 100 {
		t.Errorf("Expected 100 requests, got %d", stat.TotalRequests)
	}
	if stat.TotalErrors != 5 {
		t.Errorf("Expected 5 errors, got %d", stat.TotalErrors)
	}
	// Avg should be 1000ms / 100 = 10ms
	if stat.AvgResponseTimeMs != 10.0 {
		t.Errorf("Expected avg 10ms, got %v", stat.AvgResponseTimeMs)
	}
	if stat.MinResponseTimeMs != 5.0 {
		t.Errorf("Expected min 5ms, got %v", stat.MinResponseTimeMs)
	}
	if stat.MaxResponseTimeMs != 50.0 {
		t.Errorf("Expected max 50ms, got %v", stat.MaxResponseTimeMs)
	}
	if stat.LastRequestTime == "" {
		t.Error("Expected non-empty last request time")
	}
}

func TestAtomicOperationStat_ZeroRequests(t *testing.T) {
	aos := &AtomicOperationStat{
		OperationID: "op-1",
		SpecID:      "spec-1",
		Method:      "GET",
		Path:        "/users",
	}

	stat := aos.ToOperationStat()

	if stat.TotalRequests != 0 {
		t.Errorf("Expected 0 requests, got %d", stat.TotalRequests)
	}
	if stat.AvgResponseTimeMs != 0 {
		t.Errorf("Expected avg 0, got %v", stat.AvgResponseTimeMs)
	}
	if stat.LastRequestTime != "" {
		t.Errorf("Expected empty last request time, got %q", stat.LastRequestTime)
	}
}

func TestGlobalStatsStruct(t *testing.T) {
	stats := GlobalStats{
		TotalRequests:     1000,
		TotalErrors:       50,
		ActiveSpecs:       5,
		TotalOperations:   20,
		AvgResponseTimeMs: 15.5,
		RequestsPerSecond: 100.0,
		StartTime:         time.Now(),
		Uptime:            "1h30m",
	}

	if stats.TotalRequests != 1000 {
		t.Errorf("Expected 1000 requests, got %d", stats.TotalRequests)
	}
	if stats.TotalErrors != 50 {
		t.Errorf("Expected 50 errors, got %d", stats.TotalErrors)
	}
	if stats.ActiveSpecs != 5 {
		t.Errorf("Expected 5 active specs, got %d", stats.ActiveSpecs)
	}
}

func TestSpecStatsStruct(t *testing.T) {
	stats := SpecStats{
		SpecID:            "spec-1",
		SpecName:          "Test API",
		TotalRequests:     500,
		TotalErrors:       10,
		AvgResponseTimeMs: 20.0,
	}

	if stats.SpecID != "spec-1" {
		t.Errorf("Expected spec ID 'spec-1', got %q", stats.SpecID)
	}
	if stats.SpecName != "Test API" {
		t.Errorf("Expected spec name 'Test API', got %q", stats.SpecName)
	}
}

func TestErrorStatStruct(t *testing.T) {
	now := time.Now()
	stat := ErrorStat{
		Timestamp:   now,
		SpecID:      "spec-1",
		OperationID: "op-1",
		Path:        "/users",
		Method:      "POST",
		StatusCode:  500,
		Error:       "Internal Server Error",
	}

	if stat.StatusCode != 500 {
		t.Errorf("Expected status code 500, got %d", stat.StatusCode)
	}
	if stat.Error != "Internal Server Error" {
		t.Errorf("Expected error message 'Internal Server Error', got %q", stat.Error)
	}
	if stat.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestHourlyStatStruct(t *testing.T) {
	stat := HourlyStat{
		Hour:     "14:00",
		Requests: 100,
		Errors:   5,
	}

	if stat.Hour != "14:00" {
		t.Errorf("Expected hour '14:00', got %q", stat.Hour)
	}
	if stat.Requests != 100 {
		t.Errorf("Expected 100 requests, got %d", stat.Requests)
	}
	if stat.Errors != 5 {
		t.Errorf("Expected 5 errors, got %d", stat.Errors)
	}
}

func TestOperationStatStruct(t *testing.T) {
	stat := OperationStat{
		OperationID:       "op-1",
		SpecID:            "spec-1",
		Method:            "GET",
		Path:              "/users",
		TotalRequests:     200,
		TotalErrors:       10,
		AvgResponseTimeMs: 25.5,
		MinResponseTimeMs: 5.0,
		MaxResponseTimeMs: 100.0,
		LastRequestTime:   "2024-01-01T12:00:00Z",
	}

	if stat.OperationID != "op-1" {
		t.Errorf("Expected operation ID 'op-1', got %q", stat.OperationID)
	}
	if stat.TotalRequests != 200 {
		t.Errorf("Expected 200 requests, got %d", stat.TotalRequests)
	}
	if stat.AvgResponseTimeMs != 25.5 {
		t.Errorf("Expected avg 25.5ms, got %v", stat.AvgResponseTimeMs)
	}
}
