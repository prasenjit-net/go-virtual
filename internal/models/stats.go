package models

import (
	"sync/atomic"
	"time"
)

// GlobalStats represents global statistics
type GlobalStats struct {
	TotalRequests     int64           `json:"totalRequests"`
	TotalErrors       int64           `json:"totalErrors"`
	ActiveSpecs       int             `json:"activeSpecs"`
	TotalOperations   int             `json:"totalOperations"`
	AvgResponseTimeMs float64         `json:"avgResponseTimeMs"`
	RequestsPerSecond float64         `json:"requestsPerSecond"`
	StartTime         time.Time       `json:"startTime"`
	Uptime            string          `json:"uptime"`
	TopOperations     []OperationStat `json:"topOperations"`
	RecentErrors      []ErrorStat     `json:"recentErrors"`
	RequestsByHour    []HourlyStat    `json:"requestsByHour"`
}

// SpecStats represents statistics for a specific spec
type SpecStats struct {
	SpecID            string          `json:"specId"`
	SpecName          string          `json:"specName"`
	TotalRequests     int64           `json:"totalRequests"`
	TotalErrors       int64           `json:"totalErrors"`
	AvgResponseTimeMs float64         `json:"avgResponseTimeMs"`
	Operations        []OperationStat `json:"operations"`
}

// OperationStat represents statistics for a specific operation
type OperationStat struct {
	OperationID       string  `json:"operationId"`
	SpecID            string  `json:"specId"`
	Method            string  `json:"method"`
	Path              string  `json:"path"`
	TotalRequests     int64   `json:"totalRequests"`
	TotalErrors       int64   `json:"totalErrors"`
	AvgResponseTimeMs float64 `json:"avgResponseTimeMs"`
	MinResponseTimeMs float64 `json:"minResponseTimeMs"`
	MaxResponseTimeMs float64 `json:"maxResponseTimeMs"`
	LastRequestTime   string  `json:"lastRequestTime,omitempty"`
}

// ErrorStat represents an error occurrence
type ErrorStat struct {
	Timestamp   time.Time `json:"timestamp"`
	SpecID      string    `json:"specId"`
	OperationID string    `json:"operationId"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	StatusCode  int       `json:"statusCode"`
	Error       string    `json:"error"`
}

// HourlyStat represents hourly request statistics
type HourlyStat struct {
	Hour     string `json:"hour"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}

// AtomicOperationStat is a thread-safe version of operation statistics
type AtomicOperationStat struct {
	OperationID     string
	SpecID          string
	Method          string
	Path            string
	TotalRequests   atomic.Int64
	TotalErrors     atomic.Int64
	TotalTimeNs     atomic.Int64
	MinTimeNs       atomic.Int64
	MaxTimeNs       atomic.Int64
	LastRequestTime atomic.Value // stores time.Time
}

// ToOperationStat converts to a regular OperationStat
func (a *AtomicOperationStat) ToOperationStat() OperationStat {
	totalReqs := a.TotalRequests.Load()
	totalTimeNs := a.TotalTimeNs.Load()
	var avgMs float64
	if totalReqs > 0 {
		avgMs = float64(totalTimeNs) / float64(totalReqs) / 1e6
	}

	var lastReqTime string
	if t, ok := a.LastRequestTime.Load().(time.Time); ok && !t.IsZero() {
		lastReqTime = t.Format(time.RFC3339)
	}

	return OperationStat{
		OperationID:       a.OperationID,
		SpecID:            a.SpecID,
		Method:            a.Method,
		Path:              a.Path,
		TotalRequests:     totalReqs,
		TotalErrors:       a.TotalErrors.Load(),
		AvgResponseTimeMs: avgMs,
		MinResponseTimeMs: float64(a.MinTimeNs.Load()) / 1e6,
		MaxResponseTimeMs: float64(a.MaxTimeNs.Load()) / 1e6,
		LastRequestTime:   lastReqTime,
	}
}
