// Package metrics provides Prometheus instrumentation for go-virtual.
// All metrics are registered on the default prometheus registry using promauto.
package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts every request handled by the proxy engine.
	// Labels: spec_id, method, path (OpenAPI template, e.g. /pets/{id}), status_code.
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "govirtual_requests_total",
			Help: "Total number of requests handled by go-virtual.",
		},
		[]string{"spec_id", "method", "path", "status_code"},
	)

	// RequestDurationSeconds tracks latency as a histogram.
	// Labels: spec_id, method, path.
	RequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "govirtual_request_duration_seconds",
			Help:    "Request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"spec_id", "method", "path"},
	)

	// ActiveSpecsTotal is a gauge tracking the number of enabled API specs.
	ActiveSpecsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "govirtual_active_specs_total",
			Help: "Number of currently enabled API specs.",
		},
	)

	// ProxyRequestsTotal counts requests forwarded to real backends (proxy-recording mode).
	// Labels: spec_id, backend_uri.
	ProxyRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "govirtual_proxy_requests_total",
			Help: "Total number of requests forwarded to real backends in proxy-recording mode.",
		},
		[]string{"spec_id", "backend_uri"},
	)

	// ResponseConfigMatchesTotal counts how often each named response config is matched.
	// Labels: operation_id, config_name.
	ResponseConfigMatchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "govirtual_response_config_matches_total",
			Help: "Total number of times a named response config was matched.",
		},
		[]string{"operation_id", "config_name"},
	)

	// UnmatchedRequestsTotal counts requests that matched no registered operation.
	UnmatchedRequestsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "govirtual_unmatched_requests_total",
			Help: "Total number of requests that did not match any registered operation.",
		},
	)
)

// StatusLabel converts an HTTP status code integer to the string label used in metrics.
func StatusLabel(code int) string {
	return strconv.Itoa(code)
}
