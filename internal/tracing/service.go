package tracing

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/models"
)

// subscriber wraps a channel with closed state tracking
type subscriber struct {
	ch     chan *models.Trace
	closed bool
}

// Service manages request/response tracing
type Service struct {
	mu          sync.RWMutex
	traces      []*models.Trace
	maxTraces   int
	subscribers map[string]*subscriber
}

// NewService creates a new tracing service
func NewService(maxTraces int) *Service {
	if maxTraces <= 0 {
		maxTraces = 1000
	}

	return &Service{
		traces:      make([]*models.Trace, 0),
		maxTraces:   maxTraces,
		subscribers: make(map[string]*subscriber),
	}
}

// RecordTrace records a new trace
func (s *Service) RecordTrace(trace *models.Trace) {
	s.mu.Lock()

	// Generate ID if not set
	if trace.ID == "" {
		trace.ID = uuid.New().String()
	}

	// Set timestamp if not set
	if trace.Timestamp.IsZero() {
		trace.Timestamp = time.Now()
	}

	// Add to traces
	s.traces = append(s.traces, trace)

	// Trim if over max
	if len(s.traces) > s.maxTraces {
		s.traces = s.traces[len(s.traces)-s.maxTraces:]
	}

	// Notify subscribers (non-blocking) while holding the lock
	// This ensures we don't send to closed channels
	for _, sub := range s.subscribers {
		if !sub.closed {
			select {
			case sub.ch <- trace:
			default:
				// Channel full, skip
			}
		}
	}

	s.mu.Unlock()
}

// GetTraces returns traces matching the filter
func (s *Service) GetTraces(filter *models.TraceFilter) []*models.Trace {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.Trace, 0)

	for i := len(s.traces) - 1; i >= 0; i-- {
		trace := s.traces[i]

		// Apply filters
		if filter != nil {
			if filter.SpecID != "" && trace.SpecID != filter.SpecID {
				continue
			}
			if filter.OperationID != "" && trace.OperationID != filter.OperationID {
				continue
			}
			if filter.Method != "" && trace.Request.Method != filter.Method {
				continue
			}
			if filter.StatusCode != 0 && trace.Response.StatusCode != filter.StatusCode {
				continue
			}
			if !filter.StartTime.IsZero() && trace.Timestamp.Before(filter.StartTime) {
				continue
			}
			if !filter.EndTime.IsZero() && trace.Timestamp.After(filter.EndTime) {
				continue
			}
		}

		result = append(result, trace)

		// Apply limit
		if filter != nil && filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}

	return result
}

// GetTrace returns a single trace by ID
func (s *Service) GetTrace(id string) *models.Trace {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, trace := range s.traces {
		if trace.ID == id {
			return trace
		}
	}

	return nil
}

// ClearTraces removes all traces
func (s *Service) ClearTraces() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.traces = make([]*models.Trace, 0)
}

// ClearTracesBySpec removes traces for a specific spec
func (s *Service) ClearTracesBySpec(specID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]*models.Trace, 0)
	for _, trace := range s.traces {
		if trace.SpecID != specID {
			filtered = append(filtered, trace)
		}
	}
	s.traces = filtered
}

// Subscribe creates a subscription for live traces
func (s *Service) Subscribe() (string, chan *models.Trace) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	ch := make(chan *models.Trace, 100)
	s.subscribers[id] = &subscriber{ch: ch, closed: false}

	return id, ch
}

// Unsubscribe removes a subscription
func (s *Service) Unsubscribe(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sub, ok := s.subscribers[id]; ok {
		sub.closed = true
		close(sub.ch)
		delete(s.subscribers, id)
	}
}

// GetStats returns tracing statistics
func (s *Service) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"totalTraces":      len(s.traces),
		"maxTraces":        s.maxTraces,
		"activeSubscribers": len(s.subscribers),
	}
}
