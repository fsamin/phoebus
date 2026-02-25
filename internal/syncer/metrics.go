package syncer

import "github.com/prometheus/client_golang/prometheus"

var (
	syncJobsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoebus_sync_jobs_total",
			Help: "Total number of sync jobs by status",
		},
		[]string{"status"},
	)

	syncDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "phoebus_sync_duration_seconds",
		Help:    "Duration of content sync jobs in seconds",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
	})
)

func init() {
	prometheus.MustRegister(syncJobsTotal, syncDuration)
}
