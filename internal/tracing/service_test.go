package tracing

import (
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name           string
		maxTraces      int
		expectedMax    int
	}{
		{"positive max", 500, 500},
		{"zero max defaults to 1000", 0, 1000},
		{"negative max defaults to 1000", -1, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewService(tt.maxTraces)
			if s == nil {
				t.Fatal("NewService returned nil")
			}
			if s.maxTraces != tt.expectedMax {
				t.Errorf("Expected maxTraces %d, got %d", tt.expectedMax, s.maxTraces)
			}
		})
	}
}

func TestRecordTrace(t *testing.T) {
	s := NewService(100)

	trace := &models.Trace{
		SpecID:      "spec-1",
		OperationID: "op-1",
		Request: models.TraceRequest{
			Method: "GET",
			Path:   "/users",
		},
		Response: models.TraceResponse{
			StatusCode: 200,
		},
	}

	s.RecordTrace(trace)

	// Verify ID was generated
	if trace.ID == "" {
		t.Error("Expected trace ID to be generated")
	}

	// Verify timestamp was set
	if trace.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// Verify trace was stored
	traces := s.GetTraces(nil)
	if len(traces) != 1 {
		t.Errorf("Expected 1 trace, got %d", len(traces))
	}
}

func TestRecordTrace_MaxLimit(t *testing.T) {
	s := NewService(5)

	// Record 10 traces
	for i := 0; i < 10; i++ {
		s.RecordTrace(&models.Trace{
			SpecID: "spec-1",
			Request: models.TraceRequest{
				Method: "GET",
				Path:   "/users",
			},
		})
	}

	traces := s.GetTraces(nil)
	if len(traces) != 5 {
		t.Errorf("Expected 5 traces (max limit), got %d", len(traces))
	}
}

func TestRecordTrace_PreservesExistingID(t *testing.T) {
	s := NewService(100)

	trace := &models.Trace{
		ID:     "custom-id",
		SpecID: "spec-1",
	}

	s.RecordTrace(trace)

	if trace.ID != "custom-id" {
		t.Errorf("Expected ID to be preserved as 'custom-id', got %q", trace.ID)
	}
}

func TestRecordTrace_PreservesExistingTimestamp(t *testing.T) {
	s := NewService(100)

	customTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	trace := &models.Trace{
		SpecID:    "spec-1",
		Timestamp: customTime,
	}

	s.RecordTrace(trace)

	if !trace.Timestamp.Equal(customTime) {
		t.Errorf("Expected timestamp to be preserved, got %v", trace.Timestamp)
	}
}

func TestGetTraces_NoFilter(t *testing.T) {
	s := NewService(100)

	// Record traces
	for i := 0; i < 5; i++ {
		s.RecordTrace(&models.Trace{
			SpecID: "spec-1",
		})
	}

	traces := s.GetTraces(nil)
	if len(traces) != 5 {
		t.Errorf("Expected 5 traces, got %d", len(traces))
	}
}

func TestGetTraces_FilterBySpecID(t *testing.T) {
	s := NewService(100)

	s.RecordTrace(&models.Trace{SpecID: "spec-1"})
	s.RecordTrace(&models.Trace{SpecID: "spec-2"})
	s.RecordTrace(&models.Trace{SpecID: "spec-1"})

	filter := &models.TraceFilter{SpecID: "spec-1"}
	traces := s.GetTraces(filter)

	if len(traces) != 2 {
		t.Errorf("Expected 2 traces for spec-1, got %d", len(traces))
	}
}

func TestGetTraces_FilterByOperationID(t *testing.T) {
	s := NewService(100)

	s.RecordTrace(&models.Trace{SpecID: "spec-1", OperationID: "op-1"})
	s.RecordTrace(&models.Trace{SpecID: "spec-1", OperationID: "op-2"})
	s.RecordTrace(&models.Trace{SpecID: "spec-1", OperationID: "op-1"})

	filter := &models.TraceFilter{OperationID: "op-1"}
	traces := s.GetTraces(filter)

	if len(traces) != 2 {
		t.Errorf("Expected 2 traces for op-1, got %d", len(traces))
	}
}

func TestGetTraces_FilterByMethod(t *testing.T) {
	s := NewService(100)

	s.RecordTrace(&models.Trace{
		SpecID:  "spec-1",
		Request: models.TraceRequest{Method: "GET"},
	})
	s.RecordTrace(&models.Trace{
		SpecID:  "spec-1",
		Request: models.TraceRequest{Method: "POST"},
	})
	s.RecordTrace(&models.Trace{
		SpecID:  "spec-1",
		Request: models.TraceRequest{Method: "GET"},
	})

	filter := &models.TraceFilter{Method: "GET"}
	traces := s.GetTraces(filter)

	if len(traces) != 2 {
		t.Errorf("Expected 2 GET traces, got %d", len(traces))
	}
}

func TestGetTraces_FilterByStatusCode(t *testing.T) {
	s := NewService(100)

	s.RecordTrace(&models.Trace{
		SpecID:   "spec-1",
		Response: models.TraceResponse{StatusCode: 200},
	})
	s.RecordTrace(&models.Trace{
		SpecID:   "spec-1",
		Response: models.TraceResponse{StatusCode: 404},
	})
	s.RecordTrace(&models.Trace{
		SpecID:   "spec-1",
		Response: models.TraceResponse{StatusCode: 200},
	})

	filter := &models.TraceFilter{StatusCode: 200}
	traces := s.GetTraces(filter)

	if len(traces) != 2 {
		t.Errorf("Expected 2 traces with status 200, got %d", len(traces))
	}
}

func TestGetTraces_FilterByTimeRange(t *testing.T) {
	s := NewService(100)

	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	s.RecordTrace(&models.Trace{SpecID: "spec-1", Timestamp: twoHoursAgo})
	s.RecordTrace(&models.Trace{SpecID: "spec-1", Timestamp: hourAgo})
	s.RecordTrace(&models.Trace{SpecID: "spec-1", Timestamp: now})

	// Filter: last 90 minutes
	filter := &models.TraceFilter{
		StartTime: now.Add(-90 * time.Minute),
	}
	traces := s.GetTraces(filter)

	if len(traces) != 2 {
		t.Errorf("Expected 2 traces in last 90 minutes, got %d", len(traces))
	}
}

func TestGetTraces_FilterWithLimit(t *testing.T) {
	s := NewService(100)

	for i := 0; i < 10; i++ {
		s.RecordTrace(&models.Trace{SpecID: "spec-1"})
	}

	filter := &models.TraceFilter{Limit: 3}
	traces := s.GetTraces(filter)

	if len(traces) != 3 {
		t.Errorf("Expected 3 traces (limit), got %d", len(traces))
	}
}

func TestGetTraces_CombinedFilters(t *testing.T) {
	s := NewService(100)

	s.RecordTrace(&models.Trace{
		SpecID:   "spec-1",
		Request:  models.TraceRequest{Method: "GET"},
		Response: models.TraceResponse{StatusCode: 200},
	})
	s.RecordTrace(&models.Trace{
		SpecID:   "spec-1",
		Request:  models.TraceRequest{Method: "POST"},
		Response: models.TraceResponse{StatusCode: 200},
	})
	s.RecordTrace(&models.Trace{
		SpecID:   "spec-2",
		Request:  models.TraceRequest{Method: "GET"},
		Response: models.TraceResponse{StatusCode: 200},
	})

	filter := &models.TraceFilter{
		SpecID: "spec-1",
		Method: "GET",
	}
	traces := s.GetTraces(filter)

	if len(traces) != 1 {
		t.Errorf("Expected 1 trace matching both filters, got %d", len(traces))
	}
}

func TestGetTrace(t *testing.T) {
	s := NewService(100)

	trace := &models.Trace{
		ID:     "test-id",
		SpecID: "spec-1",
	}
	s.RecordTrace(trace)

	// Get existing trace
	result := s.GetTrace("test-id")
	if result == nil {
		t.Fatal("Expected to find trace")
	}
	if result.SpecID != "spec-1" {
		t.Errorf("Expected spec ID 'spec-1', got %q", result.SpecID)
	}

	// Get non-existent trace
	result = s.GetTrace("nonexistent")
	if result != nil {
		t.Error("Expected nil for non-existent trace")
	}
}

func TestClearTraces(t *testing.T) {
	s := NewService(100)

	for i := 0; i < 5; i++ {
		s.RecordTrace(&models.Trace{SpecID: "spec-1"})
	}

	// Verify traces exist
	if len(s.GetTraces(nil)) != 5 {
		t.Error("Expected 5 traces before clear")
	}

	s.ClearTraces()

	if len(s.GetTraces(nil)) != 0 {
		t.Error("Expected 0 traces after clear")
	}
}

func TestClearTracesBySpec(t *testing.T) {
	s := NewService(100)

	s.RecordTrace(&models.Trace{SpecID: "spec-1"})
	s.RecordTrace(&models.Trace{SpecID: "spec-1"})
	s.RecordTrace(&models.Trace{SpecID: "spec-2"})

	s.ClearTracesBySpec("spec-1")

	traces := s.GetTraces(nil)
	if len(traces) != 1 {
		t.Errorf("Expected 1 trace remaining, got %d", len(traces))
	}
	if traces[0].SpecID != "spec-2" {
		t.Errorf("Expected remaining trace to be spec-2, got %q", traces[0].SpecID)
	}
}

func TestSubscribe(t *testing.T) {
	s := NewService(100)

	id, ch := s.Subscribe()

	if id == "" {
		t.Error("Expected non-empty subscription ID")
	}
	if ch == nil {
		t.Error("Expected non-nil channel")
	}

	// Verify subscription is registered
	stats := s.GetStats()
	if stats["activeSubscribers"].(int) != 1 {
		t.Error("Expected 1 active subscriber")
	}

	// Cleanup
	s.Unsubscribe(id)
}

func TestUnsubscribe(t *testing.T) {
	s := NewService(100)

	id, _ := s.Subscribe()

	// Verify subscription exists
	stats := s.GetStats()
	if stats["activeSubscribers"].(int) != 1 {
		t.Error("Expected 1 active subscriber")
	}

	s.Unsubscribe(id)

	// Verify subscription removed
	stats = s.GetStats()
	if stats["activeSubscribers"].(int) != 0 {
		t.Error("Expected 0 active subscribers after unsubscribe")
	}

	// Unsubscribe non-existent (should not panic)
	s.Unsubscribe("nonexistent")
}

func TestSubscriberReceivesTraces(t *testing.T) {
	s := NewService(100)

	_, ch := s.Subscribe()

	// Record a trace
	s.RecordTrace(&models.Trace{SpecID: "spec-1"})

	// Should receive trace on channel
	select {
	case trace := <-ch:
		if trace.SpecID != "spec-1" {
			t.Errorf("Expected spec ID 'spec-1', got %q", trace.SpecID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for trace on subscription channel")
	}
}

func TestGetStats(t *testing.T) {
	s := NewService(500)

	// Record some traces
	for i := 0; i < 10; i++ {
		s.RecordTrace(&models.Trace{SpecID: "spec-1"})
	}

	// Subscribe
	id, _ := s.Subscribe()
	defer s.Unsubscribe(id)

	stats := s.GetStats()

	if stats["totalTraces"].(int) != 10 {
		t.Errorf("Expected 10 total traces, got %v", stats["totalTraces"])
	}
	if stats["maxTraces"].(int) != 500 {
		t.Errorf("Expected max traces 500, got %v", stats["maxTraces"])
	}
	if stats["activeSubscribers"].(int) != 1 {
		t.Errorf("Expected 1 active subscriber, got %v", stats["activeSubscribers"])
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewService(100)

	done := make(chan bool)

	// Concurrent writers
	go func() {
		for i := 0; i < 50; i++ {
			s.RecordTrace(&models.Trace{SpecID: "spec-1"})
		}
		done <- true
	}()

	// Concurrent readers
	go func() {
		for i := 0; i < 50; i++ {
			_ = s.GetTraces(nil)
		}
		done <- true
	}()

	// Concurrent subscribe/unsubscribe
	go func() {
		for i := 0; i < 10; i++ {
			id, _ := s.Subscribe()
			s.Unsubscribe(id)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify data integrity
	traces := s.GetTraces(nil)
	if len(traces) != 50 {
		t.Errorf("Expected 50 traces, got %d", len(traces))
	}
}
