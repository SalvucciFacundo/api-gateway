package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// httpRequestsTotal counts every request, labelled by method, path and status
// code (METRICS-001).
var httpRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests handled by the gateway.",
	},
	[]string{"method", "path", "status"},
)

// httpRequestDuration records request latency, labelled by method and path,
// with the 10ms..5s buckets from METRICS-002.
var httpRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
	},
	[]string{"method", "path"},
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
}

// Metrics returns middleware that records every request in the counters and
// histograms above (METRICS-005). The response writer is wrapped so the actual
// status code is captured even when the handler does not call WriteHeader
// explicitly.
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &responseWriter{ResponseWriter: w}
			next.ServeHTTP(wrapped, r)

			status := strconv.Itoa(wrapped.Status())
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
		})
	}
}
