package handler

import (
	"net/http"

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
