// Package instrumentation provides enterprise-grade observability for the
// cache/v2 platform. It includes Prometheus metrics collection, OpenTelemetry
// distributed tracing, structured JSON logging, and a health-check subsystem
// that aggregates backend liveness probes.
package instrumentation

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics collects Prometheus counters, histograms, and gauges for cache
// operations. It supports multiple backends via the "backend" label so
// that layered and multi-backend deployments produce per-store metrics.
//
// Typical usage:
//
//	m := NewMetrics("cache", "store")
//	if err := m.Register(); err != nil { ... }
//	// later:
//	m.RecordHit("redis")
//	m.RecordLatency("redis", "get", 230*time.Microsecond)
type Metrics struct {
	hits      prometheus.Counter
	misses    prometheus.Counter
	sets      prometheus.Counter
	deletes   prometheus.Counter
	errors    *prometheus.CounterVec
	evictions prometheus.Counter

	// operations counts operations by backend.
	operations *prometheus.CounterVec

	// duration tracks operation latency distributions.
	duration *prometheus.HistogramVec

	// items tracks the current item count per backend.
	items prometheus.Gauge

	// memory tracks the current memory usage per backend.
	memory prometheus.Gauge
}

// latencyBuckets defines the histogram buckets for operation latencies.
// They span from 10µs to 10s on a logarithmic scale, which is appropriate
// for both in-memory and network-backed caches.
var latencyBuckets = []float64{
	0.00001, // 10µs
	0.00005, // 50µs
	0.0001,  // 100µs
	0.0005,  // 500µs
	0.001,   // 1ms
	0.005,   // 5ms
	0.01,    // 10ms
	0.025,   // 25ms
	0.05,    // 50ms
	0.1,     // 100ms
	0.25,    // 250ms
	0.5,     // 500ms
	1.0,     // 1s
	5.0,     // 5s
	10.0,    // 10s
}

// NewMetrics creates a Metrics collector with the given Prometheus namespace
// and subsystem. For example, namespace="cache" and subsystem="store" will
// produce metrics like cache_store_hits_total.
func NewMetrics(namespace, subsystem string) *Metrics {
	const helpSuffix = "Total number of cache hits."
	m := &Metrics{
		hits: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "hits_total",
			Help:      helpSuffix,
		}),
		misses: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "misses_total",
			Help:      "Total number of cache misses.",
		}),
		sets: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "sets_total",
			Help:      "Total number of cache set operations.",
		}),
		deletes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "deletes_total",
			Help:      "Total number of cache delete operations.",
		}),
		evictions: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "evictions_total",
			Help:      "Total number of cache evictions.",
		}),
		operations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "operations_total",
			Help:      "Total number of cache operations by type and backend.",
		}, []string{"backend", "operation"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "errors_total",
			Help:      "Total number of cache errors by backend and operation.",
		}, []string{"backend", "operation"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "operation_duration_seconds",
			Help:      "Cache operation latency in seconds.",
			Buckets:   latencyBuckets,
		}, []string{"backend", "operation"}),
		items: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "items",
			Help:      "Current number of items in the cache.",
		}),
		memory: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "memory_bytes",
			Help:      "Current memory usage of the cache in bytes.",
		}),
	}
	return m
}

// Register registers all Prometheus collectors with the default registry.
// It returns an error if any collector fails to register (e.g. duplicate
// registration).
func (m *Metrics) Register() error {
	collectors := []prometheus.Collector{
		m.hits, m.misses, m.sets, m.deletes, m.evictions,
		m.operations, m.errors, m.duration, m.items, m.memory,
	}
	for _, c := range collectors {
		if err := prometheus.Register(c); err != nil {
			//nolint:wrapcheck // error is already wrapped by internal packages
			return err
		}
	}
	return nil
}

// Unregister removes all collectors from the default Prometheus registry.
// This is primarily useful in tests.
func (m *Metrics) Unregister() {
	collectors := []prometheus.Collector{
		m.hits, m.misses, m.sets, m.deletes, m.evictions,
		m.operations, m.errors, m.duration, m.items, m.memory,
	}
	for _, c := range collectors {
		prometheus.Unregister(c)
	}
}

// RecordHit increments the cache hit counter for the given backend.
func (m *Metrics) RecordHit(backend string) {
	m.hits.Inc()
	m.operations.WithLabelValues(backend, "get").Inc()
}

// RecordMiss increments the cache miss counter for the given backend.
func (m *Metrics) RecordMiss(backend string) {
	m.misses.Inc()
	m.operations.WithLabelValues(backend, "get").Inc()
}

// RecordSet increments the cache set counter for the given backend.
func (m *Metrics) RecordSet(backend string) {
	m.sets.Inc()
	m.operations.WithLabelValues(backend, "set").Inc()
}

// RecordDelete increments the cache delete counter for the given backend.
func (m *Metrics) RecordDelete(backend string) {
	m.deletes.Inc()
	m.operations.WithLabelValues(backend, "delete").Inc()
}

// RecordError increments the error counter for the given backend and
// operation type.
func (m *Metrics) RecordError(backend, op string) {
	m.errors.WithLabelValues(backend, op).Inc()
}

// RecordEviction increments the eviction counter for the given backend.
func (m *Metrics) RecordEviction(_backend string) {
	m.evictions.Inc()
}

// RecordLatency records the latency of a cache operation as a histogram
// observation. The duration is automatically converted to seconds.
func (m *Metrics) RecordLatency(backend, op string, latency time.Duration) {
	m.duration.WithLabelValues(backend, op).Observe(latency.Seconds())
}

// UpdateItems sets the current item count gauge for the given backend.
// This is typically called from a periodic stats reporter.
func (m *Metrics) UpdateItems(_ string, count int64) {
	m.items.Set(float64(count))
}

// UpdateMemory sets the current memory usage gauge for the given backend.
func (m *Metrics) UpdateMemory(_ string, bytes int64) {
	m.memory.Set(float64(bytes))
}
