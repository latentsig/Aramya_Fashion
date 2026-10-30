package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns a process-local registry so tests and multiple service instances
// never collide through Prometheus' global default registry.
type Metrics struct {
	registry      *prometheus.Registry
	requests      *prometheus.CounterVec
	requestTime   *prometheus.HistogramVec
	databaseReady prometheus.Gauge
	orderAttempts *prometheus.CounterVec
}

func New() *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aramya",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests served by the Aramya API.",
		}, []string{"method", "route", "status"}),
		requestTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "aramya",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method", "route"}),
		databaseReady: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "aramya",
			Name:      "database_ready",
			Help:      "Whether the most recent API database readiness check succeeded (1) or failed (0).",
		}),
		orderAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aramya",
			Name:      "order_creation_attempts_total",
			Help:      "Checkout order creation attempts by bounded outcome.",
		}, []string{"outcome"}),
	}
	registry.MustRegister(m.requests, m.requestTime, m.databaseReady, m.orderAttempts, prometheus.NewGoCollector())
	return m
}

// Middleware records route templates only. It never uses raw URLs, query
// strings, user identifiers, customer data, or request bodies as labels.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		m.requests.WithLabelValues(method, route, strconv.Itoa(c.Writer.Status())).Inc()
		m.requestTime.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
	}
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) SetDatabaseReady(ready bool) {
	if ready {
		m.databaseReady.Set(1)
		return
	}
	m.databaseReady.Set(0)
}

// RecordOrderAttempt accepts only known, bounded outcomes to prevent labels
// from ever containing error strings, IDs, or other untrusted values.
func (m *Metrics) RecordOrderAttempt(outcome string) {
	switch outcome {
	case "created", "replayed", "rejected", "failed":
		m.orderAttempts.WithLabelValues(outcome).Inc()
	}
}
