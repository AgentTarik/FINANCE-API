package telemetry

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTP metrics
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests received, partitioned by method, route and status class.",
		},
		[]string{"method", "route", "status_class"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds, partitioned by method, route and status class.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5},
		},
		[]string{"method", "route", "status_class"},
	)
)

// Domain metrics
var (
	transactionsProcessedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "transactions_processed_total",
			Help: "Total number of transactions successfully processed by the worker.",
		},
	)

	transactionsFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transactions_failed_total",
			Help: "Total number of transactions that failed, partitioned by reason.",
		},
		[]string{"reason"}, // reasons: validation | db | schema | kafka
	)

	workerQueueCurrent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_queue_current",
			Help: "Current number of items in the in-process worker queue (approximate).",
		},
	)
)

// User metrics
var (
	usersCreatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "users_created_total",
			Help: "Total number of user accounts successfully created.",
		},
	)

	usersCreateFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "users_create_failed_total",
			Help: "Total number of failed user creation attempts, partitioned by reason.",
		},
		[]string{"reason"}, // reasons: validation | db | conflict | unknown
	)

	usersGetTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "users_get_total",
			Help: "Total number of GET /users/{id} requests, partitioned by found (true/false).",
		},
		[]string{"found"}, // "true" | "false"
	)

	// Gauge that tracks how many users exist
	usersTotalCurrent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "users_total_current",
			Help: "Current number of user accounts known to the service.",
		},
	)
)

// InitMetrics called on startup
func InitMetrics() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDurationSeconds,
		transactionsProcessedTotal,
		transactionsFailedTotal,
		workerQueueCurrent,
		usersCreatedTotal,
		usersCreateFailedTotal,
		usersGetTotal,
		usersTotalCurrent,
	)
}

// PrometheusMiddleware measures one HTTP request: increments counter and observes latency.
// It uses gin.Context.FullPath() to record the *route template* (e.g., /v1/transactions).
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next() // execute handler chain

		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		method := c.Request.Method
		status := c.Writer.Status()
		statusClass := fmt.Sprintf("%dxx", status/100)

		httpRequestsTotal.WithLabelValues(method, route, statusClass).Inc()
		httpRequestDurationSeconds.WithLabelValues(method, route, statusClass).Observe(time.Since(start).Seconds())
	}
}

// MetricsHandler exposes /metrics in Prometheus text exposition format.
func MetricsHandler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}

