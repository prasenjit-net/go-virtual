package stats

import (
	"sort"
	"sync"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

// Collector collects and aggregates statistics
type Collector struct {
	mu            sync.RWMutex
	startTime     time.Time
	operations    map[string]*models.AtomicOperationStat // operationID -> stats
	recentErrors  []models.ErrorStat
	hourlyStats   map[string]*hourlyCounter // "YYYY-MM-DD-HH" -> counter
	maxErrors     int
	maxHourlySlots int
}

type hourlyCounter struct {
	Hour     string
	Requests int64
	Errors   int64
}

// NewCollector creates a new statistics collector
func NewCollector() *Collector {
	return &Collector{
		startTime:      time.Now(),
		operations:     make(map[string]*models.AtomicOperationStat),
		recentErrors:   make([]models.ErrorStat, 0),
		hourlyStats:    make(map[string]*hourlyCounter),
		maxErrors:      100,
		maxHourlySlots: 168, // 7 days
	}
}

// RecordRequest records a request for statistics
func (c *Collector) RecordRequest(specID, operationID, method, path string, duration time.Duration, isError bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get or create operation stats
	opStats, ok := c.operations[operationID]
	if !ok {
		opStats = &models.AtomicOperationStat{
			OperationID: operationID,
			SpecID:      specID,
			Method:      method,
			Path:        path,
		}
		opStats.MinTimeNs.Store(duration.Nanoseconds())
		c.operations[operationID] = opStats
	}

	// Update stats
	opStats.TotalRequests.Add(1)
	opStats.TotalTimeNs.Add(duration.Nanoseconds())
	opStats.LastRequestTime.Store(time.Now())

	// Update min/max
	durationNs := duration.Nanoseconds()
	for {
		currentMin := opStats.MinTimeNs.Load()
		if durationNs >= currentMin || opStats.MinTimeNs.CompareAndSwap(currentMin, durationNs) {
			break
		}
	}
	for {
		currentMax := opStats.MaxTimeNs.Load()
		if durationNs <= currentMax || opStats.MaxTimeNs.CompareAndSwap(currentMax, durationNs) {
			break
		}
	}

	if isError {
		opStats.TotalErrors.Add(1)
	}

	// Update hourly stats
	hourKey := time.Now().Format("2006-01-02-15")
	hourly, ok := c.hourlyStats[hourKey]
	if !ok {
		hourly = &hourlyCounter{Hour: hourKey}
		c.hourlyStats[hourKey] = hourly
		c.cleanupOldHourlyStats()
	}
	hourly.Requests++
	if isError {
		hourly.Errors++
	}
}

// RecordError records an error
func (c *Collector) RecordError(specID, operationID, path, method string, statusCode int, err string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	errorStat := models.ErrorStat{
		Timestamp:   time.Now(),
		SpecID:      specID,
		OperationID: operationID,
		Path:        path,
		Method:      method,
		StatusCode:  statusCode,
		Error:       err,
	}

	c.recentErrors = append(c.recentErrors, errorStat)
	if len(c.recentErrors) > c.maxErrors {
		c.recentErrors = c.recentErrors[1:]
	}
}

// cleanupOldHourlyStats removes hourly stats older than maxHourlySlots
func (c *Collector) cleanupOldHourlyStats() {
	if len(c.hourlyStats) <= c.maxHourlySlots {
		return
	}

	// Get sorted keys
	keys := make([]string, 0, len(c.hourlyStats))
	for k := range c.hourlyStats {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Remove oldest entries
	toRemove := len(keys) - c.maxHourlySlots
	for i := 0; i < toRemove; i++ {
		delete(c.hourlyStats, keys[i])
	}
}

// GetGlobalStats returns global statistics
func (c *Collector) GetGlobalStats(activeSpecs, totalOperations int) *models.GlobalStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalRequests, totalErrors, totalTimeNs int64

	opStats := make([]models.OperationStat, 0, len(c.operations))
	for _, op := range c.operations {
		stat := op.ToOperationStat()
		opStats = append(opStats, stat)
		totalRequests += stat.TotalRequests
		totalErrors += stat.TotalErrors
		totalTimeNs += op.TotalTimeNs.Load()
	}

	// Sort by total requests (descending)
	sort.Slice(opStats, func(i, j int) bool {
		return opStats[i].TotalRequests > opStats[j].TotalRequests
	})

	// Top 10 operations
	topOps := opStats
	if len(topOps) > 10 {
		topOps = topOps[:10]
	}

	// Calculate average response time
	var avgResponseTimeMs float64
	if totalRequests > 0 {
		avgResponseTimeMs = float64(totalTimeNs) / float64(totalRequests) / 1e6
	}

	// Calculate requests per second
	uptime := time.Since(c.startTime).Seconds()
	var requestsPerSecond float64
	if uptime > 0 {
		requestsPerSecond = float64(totalRequests) / uptime
	}

	// Build hourly stats
	hourlyStats := c.buildHourlyStats()

	return &models.GlobalStats{
		TotalRequests:     totalRequests,
		TotalErrors:       totalErrors,
		ActiveSpecs:       activeSpecs,
		TotalOperations:   totalOperations,
		AvgResponseTimeMs: avgResponseTimeMs,
		RequestsPerSecond: requestsPerSecond,
		StartTime:         c.startTime,
		Uptime:            formatDuration(time.Since(c.startTime)),
		TopOperations:     topOps,
		RecentErrors:      c.recentErrors,
		RequestsByHour:    hourlyStats,
	}
}

// GetSpecStats returns statistics for a specific spec
func (c *Collector) GetSpecStats(specID, specName string) *models.SpecStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalRequests, totalErrors, totalTimeNs int64
	opStats := make([]models.OperationStat, 0)

	for _, op := range c.operations {
		if op.SpecID != specID {
			continue
		}

		stat := op.ToOperationStat()
		opStats = append(opStats, stat)
		totalRequests += stat.TotalRequests
		totalErrors += stat.TotalErrors
		totalTimeNs += op.TotalTimeNs.Load()
	}

	var avgResponseTimeMs float64
	if totalRequests > 0 {
		avgResponseTimeMs = float64(totalTimeNs) / float64(totalRequests) / 1e6
	}

	return &models.SpecStats{
		SpecID:            specID,
		SpecName:          specName,
		TotalRequests:     totalRequests,
		TotalErrors:       totalErrors,
		AvgResponseTimeMs: avgResponseTimeMs,
		Operations:        opStats,
	}
}

// GetOperationStats returns statistics for a specific operation
func (c *Collector) GetOperationStats(operationID string) *models.OperationStat {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if op, ok := c.operations[operationID]; ok {
		stat := op.ToOperationStat()
		return &stat
	}

	return nil
}

// buildHourlyStats builds the hourly statistics array
func (c *Collector) buildHourlyStats() []models.HourlyStat {
	// Get sorted keys for the last 24 hours
	now := time.Now()
	stats := make([]models.HourlyStat, 0, 24)

	for i := 23; i >= 0; i-- {
		hour := now.Add(-time.Duration(i) * time.Hour)
		hourKey := hour.Format("2006-01-02-15")
		
		stat := models.HourlyStat{
			Hour: hour.Format("15:00"),
		}
		
		if hourly, ok := c.hourlyStats[hourKey]; ok {
			stat.Requests = hourly.Requests
			stat.Errors = hourly.Errors
		}
		
		stats = append(stats, stat)
	}

	return stats
}

// Reset resets all statistics
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.startTime = time.Now()
	c.operations = make(map[string]*models.AtomicOperationStat)
	c.recentErrors = make([]models.ErrorStat, 0)
	c.hourlyStats = make(map[string]*hourlyCounter)
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return time.Duration(d).Round(time.Minute).String()
	}
	if hours > 0 {
		return time.Duration(d).Round(time.Minute).String()
	}
	if minutes > 0 {
		return time.Duration(d).Round(time.Second).String()
	}
	return time.Duration(d).Round(time.Millisecond).String()
}
