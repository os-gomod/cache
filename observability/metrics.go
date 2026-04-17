package observability

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/os-gomod/cache/internal/stringutil"
)

const (
	promNamespace = "cache"
)

// PrometheusInterceptor collects Prometheus metrics for cache operations.
type PrometheusInterceptor struct {
	hitCounter   *prometheus.CounterVec
	missCounter  *prometheus.CounterVec
	errorCounter *prometheus.CounterVec
	latency      *prometheus.HistogramVec
	bytesRead    *prometheus.HistogramVec
}

// PrometheusConfig holds optional configuration for the Prometheus interceptor.
type PrometheusConfig struct {
	Namespace string
}

// NewPrometheusInterceptor creates a new Prometheus metrics interceptor.
// Returns an error if reg is nil.
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
		Buckets:   prometheus.ExponentialBuckets(64, 4, 8),
	}, labelNames)
	p := &PrometheusInterceptor{
		hitCounter:   hitCounter,
		missCounter:  missCounter,
		errorCounter: errorCounter,
		latency:      latency,
		bytesRead:    bytesRead,
	}
	for _, col := range []prometheus.Collector{
		hitCounter, missCounter, errorCounter, latency, bytesRead,
	} {
		if err := reg.Register(col); err != nil {
			_ = err // already registered is acceptable
		}
	}
	return p, nil
}

// Before returns the context unchanged. Metrics are recorded in After.
func (p *PrometheusInterceptor) Before(ctx context.Context, _ Op) context.Context {
	return ctx
}

// After records Prometheus metrics for the completed cache operation.
func (p *PrometheusInterceptor) After(_ context.Context, op Op, result Result) {
	labels := prometheus.Labels{
		"backend": op.Backend,
		"op":      op.Name,
	}
	if result.Latency > 0 {
		p.latency.With(labels).Observe(result.Latency.Seconds())
	}
	if stringutil.IsReadOp(op.Name) {
		if result.Hit {
			p.hitCounter.With(labels).Inc()
		} else {
			p.missCounter.With(labels).Inc()
		}
	}
	if result.ByteSize > 0 && stringutil.IsReadOp(op.Name) {
		p.bytesRead.With(labels).Observe(float64(result.ByteSize))
	}
	if result.Err != nil {
		p.errorCounter.With(labels).Inc()
	}
}

// MustNewPrometheusInterceptor is like NewPrometheusInterceptor but panics on error.
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
