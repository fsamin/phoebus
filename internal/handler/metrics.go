package handler

import (
	"math"
	"net/http"
	"sort"
	"strings"
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
		Help: "Number of active users (authenticated in last 15 min)",
	})

	exerciseAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoebus_exercise_attempts_total",
			Help: "Total number of exercise attempts",
		},
		[]string{"type", "correct"},
	)

	stepsCompletedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "phoebus_steps_completed_total",
		Help: "Total number of steps marked as completed",
	})

	learnersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "phoebus_learners_total",
		Help: "Total number of registered learners",
	})

	learningPathsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "phoebus_learning_paths_total",
		Help: "Total number of active learning paths",
	})
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		activeUsers,
		exerciseAttemptsTotal,
		stepsCompletedTotal,
		learnersTotal,
		learningPathsTotal,
	)
}

// Metrics returns the Prometheus metrics handler.
func (h *Handler) Metrics() http.Handler {
	return promhttp.Handler()
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// normalizePath collapses UUID/ID segments to keep cardinality low.
func normalizePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if len(part) >= 32 || isNumeric(part) {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
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

// updateGauges periodically queries the DB to update gauge metrics.
func (h *Handler) updateGauges() {
	for {
		var count int
		if err := h.db.Get(&count, `SELECT COUNT(*) FROM users WHERE role = 'learner'`); err == nil {
			learnersTotal.Set(float64(count))
		}
		if err := h.db.Get(&count, `SELECT COUNT(*) FROM learning_paths WHERE deleted_at IS NULL`); err == nil {
			learningPathsTotal.Set(float64(count))
		}
		if err := h.db.Get(&count, `SELECT COUNT(*) FROM users WHERE updated_at > now() - interval '15 minutes'`); err == nil {
			activeUsers.Set(float64(count))
		}
		time.Sleep(30 * time.Second)
	}
}
