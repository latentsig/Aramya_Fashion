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
	registry               *prometheus.Registry
	requests               *prometheus.CounterVec
	requestTime            *prometheus.HistogramVec
	databaseReady          prometheus.Gauge
	orderAttempts          *prometheus.CounterVec
	inventoryReservations  *prometheus.CounterVec
	paymentWebhooks        *prometheus.CounterVec
	paymentReconciliations *prometheus.CounterVec
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
		inventoryReservations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aramya",
			Name:      "inventory_reservation_attempts_total",
			Help:      "Tracked inventory reservation attempts by bounded outcome.",
		}, []string{"outcome"}),
		paymentWebhooks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aramya",
			Name:      "payment_webhook_events_total",
			Help:      "Razorpay webhook deliveries by bounded processing outcome.",
		}, []string{"outcome"}),
		paymentReconciliations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aramya",
			Name:      "payment_reconciliation_attempts_total",
			Help:      "Aged Razorpay payment reconciliation outcomes.",
		}, []string{"outcome"}),
	}
	registry.MustRegister(m.requests, m.requestTime, m.databaseReady, m.orderAttempts, m.inventoryReservations, m.paymentWebhooks, m.paymentReconciliations, prometheus.NewGoCollector())
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

// RecordInventoryReservation accepts only fixed outcomes, keeping metrics free
// of identifiers, PII, SQL errors, and other untrusted labels.
func (m *Metrics) RecordInventoryReservation(outcome string) {
	switch outcome {
	case "reserved", "rejected", "failed":
		m.inventoryReservations.WithLabelValues(outcome).Inc()
	}
}

// RecordPaymentWebhook accepts only fixed outcome labels, never event IDs,
// payload fields, customer details, or error strings.
func (m *Metrics) RecordPaymentWebhook(outcome string) {
	switch outcome {
	case "processed", "duplicate", "invalid_signature", "invalid_payload", "conflict", "failed", "rejected_headers", "too_large":
		m.paymentWebhooks.WithLabelValues(outcome).Inc()
	}
}

func (m *Metrics) RecordPaymentReconciliation(outcome string) {
	switch outcome {
	case "captured", "payment_failed", "pending", "error":
		m.paymentReconciliations.WithLabelValues(outcome).Inc()
	}
}
