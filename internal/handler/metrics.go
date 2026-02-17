package handler

import (
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoebus_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoebus_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	activeUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "phoebus_active_users",
		Help: "Number of active users",
	})

	syncJobsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoebus_sync_jobs_total",
			Help: "Total number of sync jobs by status",
		},
		[]string{"status"},
	)

	exerciseAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoebus_exercise_attempts_total",
			Help: "Total number of exercise attempts",
		},
		[]string{"correct"},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		activeUsers,
		syncJobsTotal,
		exerciseAttemptsTotal,
	)
}

// Metrics returns the Prometheus metrics handler.
func (h *Handler) Metrics() http.Handler {
	return promhttp.Handler()
}

// latencyTracker keeps recent request durations for percentile computation.
var latencyTracker = &latencyRing{maxSize: 1000}

type latencyRing struct {
	mu      sync.Mutex
	samples []time.Duration
	maxSize int
}

func (lr *latencyRing) Record(d time.Duration) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if len(lr.samples) >= lr.maxSize {
		lr.samples = lr.samples[1:]
	}
	lr.samples = append(lr.samples, d)
}

func (lr *latencyRing) Percentiles() (p50, p95, p99 float64) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	n := len(lr.samples)
	if n == 0 {
		return 0, 0, 0
	}
	sorted := make([]float64, n)
	for i, d := range lr.samples {
		sorted[i] = float64(d.Milliseconds())
	}
	sort.Float64s(sorted)
	pct := func(p float64) float64 {
		idx := int(math.Ceil(p*float64(n))) - 1
		if idx < 0 {
			idx = 0
		}
		return sorted[idx]
	}
	return pct(0.50), pct(0.95), pct(0.99)
}
