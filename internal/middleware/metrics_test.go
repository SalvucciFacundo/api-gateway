package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestMetricsRecordsCounter verifies METRICS-001: a completed request
// increments the counter labelled by method, path and status.
func TestMetricsRecordsCounter(t *testing.T) {
	const path = "/api/v1/test-metrics-counter"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	Metrics()(handler).ServeHTTP(rec, req)

	got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(http.MethodGet, path, "200"))
	if got != 1 {
		t.Errorf("http_requests_total{method=GET,path=%s,status=200} = %v, want 1", path, got)
	}
}

// TestMetricsRecordsDistinctStatuses verifies each status code is tracked in
// its own counter entry.
func TestMetricsRecordsDistinctStatuses(t *testing.T) {
	const path = "/api/v1/test-metrics-status"
	for _, code := range []int{http.StatusOK, http.StatusUnauthorized, http.StatusInternalServerError} {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		})
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		Metrics()(handler).ServeHTTP(rec, req)
	}

	for status, want := range map[string]float64{"200": 1, "401": 1, "500": 1} {
		if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(http.MethodGet, path, status)); got != want {
			t.Errorf("http_requests_total{method=GET,path=%s,status=%s} = %v, want %v", path, status, got, want)
		}
	}
}

// TestMetricsRecordsHistogram verifies METRICS-002: the request latency is
// observed into the histogram for the request's method and path.
func TestMetricsRecordsHistogram(t *testing.T) {
	const path = "/api/v1/test-metrics-histogram"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	Metrics()(handler).ServeHTTP(rec, req)

	got := histogramSampleCount(http.MethodGet, path)
	if got != 1 {
		t.Errorf("http_request_duration_seconds{method=GET,path=%s} sample count = %d, want 1", path, got)
	}
}

// histogramSampleCount returns the number of observations recorded in the
// http_request_duration_seconds histogram for the given method and path.
func histogramSampleCount(method, path string) uint64 {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return 0
	}
	for _, mf := range mfs {
		if mf.GetName() != "http_request_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			var methodVal, pathVal string
			for _, lp := range m.GetLabel() {
				switch lp.GetName() {
				case "method":
					methodVal = lp.GetValue()
				case "path":
					pathVal = lp.GetValue()
				}
			}
			if methodVal == method && pathVal == path {
				return m.GetHistogram().GetSampleCount()
			}
		}
	}
	return 0
}
