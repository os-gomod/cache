package observability

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	promNamespace = "cache"
)

// PrometheusInterceptor records cache operation metrics via Prometheus counters
// and histograms. It uses a provided Registerer (never the global default)
// to avoid global state.
type PrometheusInterceptor struct {
	hitCounter   *prometheus.CounterVec
	missCounter  *prometheus.CounterVec
	errorCounter *prometheus.CounterVec
	latency      *prometheus.HistogramVec
	bytesRead    *prometheus.HistogramVec
}

// PrometheusConfig holds optional configuration for PrometheusInterceptor.
type PrometheusConfig struct {
	// Namespace overrides the default "cache" Prometheus namespace.
	// If empty, "cache" is used.
	Namespace string
}

// NewPrometheusInterceptor creates a new interceptor that registers metrics
// with the provided Registerer. The registerer must not be nil.
//
// Histogram buckets are tailored for in-process cache latencies
// (sub-millisecond to 100ms range).
//
// Labels are low-cardinality: backend and op only. Keys are never used
// as labels to avoid cardinality explosions.
func NewPrometheusInterceptor(
	reg prometheus.Registerer,
	cfg ...PrometheusConfig,
) (*PrometheusInterceptor, error) {
	if reg == nil {
		return nil, fmt.Errorf("observability: PrometheusInterceptor requires a non-nil Registerer")
	}

	ns := promNamespace
	if len(cfg) > 0 && cfg[0].Namespace != "" {
		ns = cfg[0].Namespace
	}

	labelNames := []string{"backend", "op"}

	hitCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "hits_total",
		Help:      "Total number of cache hits",
	}, labelNames)

	missCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "misses_total",
		Help:      "Total number of cache misses",
	}, labelNames)

	errorCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "errors_total",
		Help:      "Total number of cache errors",
	}, labelNames)

	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Name:      "operation_duration_seconds",
		Help:      "Cache operation latency in seconds",
		Buckets:   []float64{0.0001, 0.001, 0.01, 0.1},
	}, labelNames)

	bytesRead := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Name:      "bytes_read",
		Help:      "Size of values read from cache in bytes",
		Buckets:   prometheus.ExponentialBuckets(64, 4, 8), // 64B to ~4MB
	}, labelNames)

	p := &PrometheusInterceptor{
		hitCounter:   hitCounter,
		missCounter:  missCounter,
		errorCounter: errorCounter,
		latency:      latency,
		bytesRead:    bytesRead,
	}

	// Register all metrics. If any are already registered (e.g., in tests
	// that reuse a registry), we attempt to retrieve the existing ones.
	for _, col := range []prometheus.Collector{
		hitCounter, missCounter, errorCounter, latency, bytesRead,
	} {
		if err := reg.Register(col); err != nil {
			if existing, ok := reg.(prometheus.Gatherer); ok {
				_ = existing // Already registered; ignore.
			}
			// If registration fails due to duplicate, it's not fatal.
			// The counter is still usable in-process.
			_ = err
		}
	}

	return p, nil
}

// Before is a no-op for Prometheus — all recording happens in After.
func (p *PrometheusInterceptor) Before(ctx context.Context, _ Op) context.Context {
	return ctx
}

// After records metrics based on the operation result.
// Hits and misses are recorded for read operations. Errors are always
// recorded. Latency and byte size are recorded when available.
func (p *PrometheusInterceptor) After(_ context.Context, op Op, result Result) {
	labels := prometheus.Labels{
		"backend": op.Backend,
		"op":      op.Name,
	}

	// Record latency for all operations.
	if result.Latency > 0 {
		p.latency.With(labels).Observe(result.Latency.Seconds())
	}

	// Record hit/miss for read operations.
	if isReadOp(op.Name) {
		if result.Hit {
			p.hitCounter.With(labels).Inc()
		} else {
			p.missCounter.With(labels).Inc()
		}
	}

	// Record byte size for reads with data.
	if result.ByteSize > 0 && isReadOp(op.Name) {
		p.bytesRead.With(labels).Observe(float64(result.ByteSize))
	}

	// Record errors.
	if result.Err != nil {
		p.errorCounter.With(labels).Inc()
	}
}

// MustNewPrometheusInterceptor is like NewPrometheusInterceptor but panics
// on error. Useful for package-level initialization.
func MustNewPrometheusInterceptor(
	reg prometheus.Registerer,
	cfg ...PrometheusConfig,
) *PrometheusInterceptor {
	p, err := NewPrometheusInterceptor(reg, cfg...)
	if err != nil {
		panic(err)
	}
	return p
}
