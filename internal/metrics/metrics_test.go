package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/prasenjit/go-virtual/internal/metrics"
)

func TestRequestsTotal(t *testing.T) {
	// Record two requests with different labels
	metrics.RequestsTotal.WithLabelValues("spec1", "GET", "/pets", "200").Add(3)
	metrics.RequestsTotal.WithLabelValues("spec1", "GET", "/pets", "500").Add(1)

	// Verify counts
	c200 := testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("spec1", "GET", "/pets", "200"))
	c500 := testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("spec1", "GET", "/pets", "500"))

	if c200 < 3 {
		t.Errorf("expected count >= 3 for 200, got %v", c200)
	}
	if c500 < 1 {
		t.Errorf("expected count >= 1 for 500, got %v", c500)
	}
}

func TestRequestDurationSeconds(t *testing.T) {
	obs := metrics.RequestDurationSeconds.WithLabelValues("spec1", "POST", "/pets")
	obs.Observe(0.025)
	obs.Observe(0.150)

	// Just verify it doesn't panic and the observer exists
	if obs == nil {
		t.Error("expected non-nil observer")
	}
}

func TestActiveSpecsTotal(t *testing.T) {
	metrics.ActiveSpecsTotal.Set(5)
	got := testutil.ToFloat64(metrics.ActiveSpecsTotal)
	if got != 5 {
		t.Errorf("expected 5, got %v", got)
	}

	metrics.ActiveSpecsTotal.Set(3)
	got = testutil.ToFloat64(metrics.ActiveSpecsTotal)
	if got != 3 {
		t.Errorf("expected 3, got %v", got)
	}
}

func TestProxyRequestsTotal(t *testing.T) {
	metrics.ProxyRequestsTotal.WithLabelValues("spec2", "http://backend:8080").Inc()
	got := testutil.ToFloat64(
		metrics.ProxyRequestsTotal.WithLabelValues("spec2", "http://backend:8080"),
	)
	if got < 1 {
		t.Errorf("expected count >= 1, got %v", got)
	}
}

func TestResponseConfigMatchesTotal(t *testing.T) {
	metrics.ResponseConfigMatchesTotal.WithLabelValues("op1", "default-200").Add(7)
	got := testutil.ToFloat64(
		metrics.ResponseConfigMatchesTotal.WithLabelValues("op1", "default-200"),
	)
	if got < 7 {
		t.Errorf("expected count >= 7, got %v", got)
	}
}

func TestUnmatchedRequestsTotal(t *testing.T) {
	before := testutil.ToFloat64(metrics.UnmatchedRequestsTotal)
	metrics.UnmatchedRequestsTotal.Inc()
	after := testutil.ToFloat64(metrics.UnmatchedRequestsTotal)
	if after-before < 1 {
		t.Errorf("expected unmatched counter to increase by at least 1")
	}
}

func TestStatusLabel(t *testing.T) {
	if metrics.StatusLabel(200) != "200" {
		t.Errorf("expected '200', got %q", metrics.StatusLabel(200))
	}
	if metrics.StatusLabel(404) != "404" {
		t.Errorf("expected '404', got %q", metrics.StatusLabel(404))
	}
}
