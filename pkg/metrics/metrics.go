package metrics

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus collectors for AetherStream
type Metrics struct {
	Registry *prometheus.Registry

	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPRequestDuration   *prometheus.HistogramVec
	ActiveStreams         prometheus.Gauge
	DBConnections         prometheus.Gauge
	MemoryUsage           prometheus.Gauge
	DBQueriesTotal        *prometheus.CounterVec
	CacheHitsTotal        *prometheus.CounterVec
}

// NewMetrics creates and registers all Prometheus collectors
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		Registry: reg,
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
		ActiveStreams: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "active_streams_total",
			Help: "Number of currently active streams",
		}),
		DBConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "db_connections",
			Help: "Number of active database connections",
		}),
		MemoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "memory_bytes",
			Help: "Current memory usage in bytes",
		}),
		DBQueriesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		}, []string{"query_type"}),
		CacheHitsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		}, []string{"cache_type"}),
	}

	reg.MustRegister(m.HTTPRequestsTotal)
	reg.MustRegister(m.HTTPRequestDuration)
	reg.MustRegister(m.ActiveStreams)
	reg.MustRegister(m.DBConnections)
	reg.MustRegister(m.MemoryUsage)
	reg.MustRegister(m.DBQueriesTotal)
	reg.MustRegister(m.CacheHitsTotal)

	// Register Go runtime metrics
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	return m
}

// EchoMiddleware returns Echo middleware that records request metrics
func (m *Metrics) EchoMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start).Seconds()

			method := c.Request().Method
			path := c.Path()
			if path == "" {
				path = "unknown"
			}
			status := strconv.Itoa(c.Response().Status)

			m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
			m.HTTPRequestDuration.WithLabelValues(method, path, status).Observe(duration)

			return err
		}
	}
}

// UpdateMemory updates the memory usage gauge from runtime stats
func (m *Metrics) UpdateMemory() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.MemoryUsage.Set(float64(ms.Alloc))
}

// MetricsHandler returns an http.Handler for the /metrics endpoint
func (m *Metrics) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}
