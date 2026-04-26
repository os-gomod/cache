// Package main demonstrates observability features with os-gomod/cache v2.
//
// This example shows:
//   - Creating a cache with Prometheus metrics middleware
//   - Creating a cache with OpenTelemetry tracing middleware
//   - Creating a cache with structured logging middleware
//   - Health checks across multiple backends
//   - Statistics collection and reporting
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-gomod/cache/v2"
	"github.com/os-gomod/cache/v2/internal/middleware"
	"github.com/os-gomod/cache/v2/memory"
)

// structuredLogger is a simple structured logging middleware.
// In production, this would integrate with slog, zap, or zerolog.
type structuredLogger struct {
	component string
}

func newStructuredLogger(component string) *structuredLogger {
	return &structuredLogger{component: component}
}

func (l *structuredLogger) Log(entry middleware.LogEntry) {
	l.logOperation(entry.Op, entry.Key, entry.Latency, entry.Err)
}

func (l *structuredLogger) logOperation(op, key string, duration time.Duration, err error) {
	status := "OK"
	if err != nil {
		status = "ERROR: " + err.Error()
	}
	log.Printf("[%s] %s key=%s duration=%v status=%s",
		l.component, op, key, duration, status)
}

// prometheusRecorder is a mock Prometheus metrics recorder.
// In production, this would use github.com/prometheus/client_golang.
type prometheusRecorder struct {
	hits       int64
	misses     int64
	errors     int64
	operations int64
}

func newPrometheusRecorder() *prometheusRecorder {
	return &prometheusRecorder{}
}

func (r *prometheusRecorder) RecordHit(operation, backend string) {
	r.hits++
	r.operations++
}

func (r *prometheusRecorder) RecordMiss(operation, backend string) {
	r.misses++
	r.operations++
}

func (r *prometheusRecorder) RecordError(operation, backend string, err error) {
	r.errors++
	r.operations++
}

func (r *prometheusRecorder) RecordLatency(operation, backend string, duration time.Duration) {
	// In production: histogram.Observe(duration.Seconds())
}

// Record implements middleware.MetricsRecorder.
func (r *prometheusRecorder) Record(op string, latency time.Duration, err error) {
	r.operations++
	if err != nil {
		r.errors++
	}
}

func (r *prometheusRecorder) Summary() string {
	hitRate := float64(0)
	if r.hits+r.misses > 0 {
		hitRate = float64(r.hits) / float64(r.hits+r.misses) * 100
	}
	return fmt.Sprintf("ops=%d hits=%d misses=%d errors=%d hit_rate=%.1f%%",
		r.operations, r.hits, r.misses, r.errors, hitRate)
}

func main() {
	ctx := context.Background()

	// ---------------------------------------------------------------------------
	// Prometheus Metrics
	// ---------------------------------------------------------------------------
	fmt.Println("=== Cache with Prometheus Metrics ===")

	recorder := newPrometheusRecorder()
	backend, err := cache.NewMemory(memory.WithMaxEntries(1000))
	if err != nil {
		log.Fatalf("failed to create cache: %v", err)
	}
	defer backend.Close(ctx)

	metricsBackend := cache.WithMiddleware(backend,
		middleware.MetricsMiddleware(recorder),
	)

	// Generate some traffic
	for i := range 100 {
		key := fmt.Sprintf("item:%d", i)
		metricsBackend.Set(
			ctx,
			key,
			[]byte(fmt.Sprintf("value-%d", i)),
			5*time.Minute,
		) // nolint: errcheck
	}

	// 80 hits
	for i := range 80 {
		key := fmt.Sprintf("item:%d", i)
		_, _ = metricsBackend.Get(ctx, key)
	}

	// 20 misses
	for i := 100; i < 120; i++ {
		key := fmt.Sprintf("item:%d", i)
		_, _ = metricsBackend.Get(ctx, key)
	}

	fmt.Printf("Prometheus Metrics: %s\n", recorder.Summary())

	// ---------------------------------------------------------------------------
	// OpenTelemetry Tracing
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Cache with OpenTelemetry Tracing ===")

	// In production, create a tracer provider:
	//
	//   tp := trace.NewTracerProvider(
	//       trace.WithBatcher(exporter),
	//       trace.WithResource(resource.NewWithAttributes(
	//           semconv.SchemaURL,
	//           semconv.ServiceNameKey.String("cache-service"),
	//       )),
	//   )
	//   tracer := tp.Tracer("github.com/os-gomod/cache/v2")
	//
	// Then wrap with tracing middleware:
	//
	//   tracingBackend := cache.WithMiddleware(backend,
	//       middleware.TracingMiddleware(tracer),
	//   )
	//
	// Each cache operation will create a span with:
	//   - cache.operation: "get", "set", "delete", etc.
	//   - cache.key: the key being accessed
	//   - cache.backend: "memory", "redis", "layered"
	//   - cache.hit: true/false
	//   - cache.error: error message if failed

	fmt.Println("OpenTelemetry tracing configuration:")
	fmt.Println("  Tracer: cache-service")
	fmt.Println("  Spans created for: get, set, delete, exists, ttl")
	fmt.Println(
		"  Span attributes: cache.operation, cache.key, cache.backend, cache.hit, cache.error",
	)
	fmt.Println("  (Enable by providing a real tracer provider)")

	// ---------------------------------------------------------------------------
	// Structured Logging
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Cache with Structured Logging ===")

	logger := newStructuredLogger("cache")
	loggingBackend := cache.WithMiddleware(backend,
		middleware.LoggingMiddleware(logger),
	)

	fmt.Println("Performing operations with structured logging...")

	start := time.Now()
	loggingBackend.Set(ctx, "logged:key", []byte("logged-value"), 5*time.Minute) // nolint: errcheck
	logger.logOperation("SET", "logged:key", time.Since(start), nil)

	start = time.Now()
	val, err := loggingBackend.Get(ctx, "logged:key")
	logger.logOperation("GET", "logged:key", time.Since(start), err)
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	fmt.Printf("  Retrieved: %s\n", string(val))

	start = time.Now()
	_, err = loggingBackend.Get(ctx, "logged:nonexistent")
	logger.logOperation("GET", "logged:nonexistent", time.Since(start), err)
	if err != nil {
		fmt.Printf("  Expected miss: %v\n", err)
	}

	// ---------------------------------------------------------------------------
	// Health Checks (Multi-Backend)
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Health Checks ===")

	// Create a multi-backend manager
	localCache, _ := cache.NewMemory(memory.WithMaxEntries(100))
	defer localCache.Close(ctx)

	mgr, err := cache.NewManager(
		cache.WithNamedBackend("primary", backend),
		cache.WithNamedBackend("local", localCache),
		cache.WithDefaultBackend(backend),
	)
	if err != nil {
		log.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close(ctx)

	health := mgr.HealthCheck(ctx)
	fmt.Println("Health check results:")
	for name, err := range health {
		status := "healthy"
		if err != nil {
			status = fmt.Sprintf("unhealthy: %v", err)
		}
		fmt.Printf("  %s: %s\n", name, status)
	}

	// ---------------------------------------------------------------------------
	// Statistics Collection
	// ---------------------------------------------------------------------------
	fmt.Println("\n=== Statistics Collection ===")

	// Get stats from individual backends
	fmt.Println("Primary backend stats:")
	stats := mgr.Stats()["primary"]
	fmt.Printf("  Hits:     %d\n", stats.Hits)
	fmt.Printf("  Misses:   %d\n", stats.Misses)
	fmt.Printf("  Sets:     %d\n", stats.Sets)
	fmt.Printf("  Deletes:  %d\n", stats.Deletes)
	fmt.Printf("  Evictions:%d\n", stats.Evictions)
	fmt.Printf("  Size:     %d\n", stats.Items)

	// Get all stats at once
	allStats := mgr.Stats()
	fmt.Println("\nAll backend stats:")
	for name, s := range allStats {
		fmt.Printf("  %s: hits=%d misses=%d size=%d\n",
			name, s.Hits, s.Misses, s.Items)
	}

	fmt.Println("\n=== All Operations Completed Successfully ===")
}

// Ensure unused import is referenced.
var _ = middleware.Middleware(nil)
