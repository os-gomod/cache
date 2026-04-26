package instrumentation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// HealthStatus represents the aggregated health of all registered cache
// backends. A single unhealthy backend degrades the overall status to
// "unhealthy" if it is the primary store, or "degraded" if a fallback
// exists.
type HealthStatus struct {
	// Status is one of "healthy", "degraded", or "unhealthy".
	Status string `json:"status"`

	// Details provides per-backend status messages. The key is the
	// backend name; the value is "ok" or an error description.
	Details map[string]string `json:"details"`

	// CheckedAt is the timestamp of this health check.
	CheckedAt time.Time `json:"checked_at"`
}

// Checker aggregates health probes across multiple cache backends. Each
// registered backend must implement contracts.HealthChecker (i.e. provide a
// Check method).
type Checker struct {
	mu       sync.RWMutex
	backends map[string]contracts.HealthChecker
	timeout  time.Duration
}

// NewChecker creates a health checker with the given per-backend timeout.
// If a backend's Check method does not return within the timeout, it is
// marked as unhealthy.
func NewChecker(timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Checker{
		backends: make(map[string]contracts.HealthChecker),
		timeout:  timeout,
	}
}

// Register adds a named backend to the health checker. If a backend with
// the same name already exists, it is replaced.
func (c *Checker) Register(name string, backend contracts.HealthChecker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.backends[name] = backend
}

// Unregister removes a named backend from the health checker.
func (c *Checker) Unregister(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.backends, name)
}

// Check performs a health probe on all registered backends concurrently.
// It returns an aggregated HealthStatus where:
//
//   - "healthy": all backends passed.
//   - "degraded": at least one backend failed but others are healthy.
//   - "unhealthy": all backends failed.
//
// Each backend's Check is called with a timeout derived from c.timeout.
func (c *Checker) Check(ctx context.Context) HealthStatus {
	c.mu.RLock()
	names := make([]string, 0, len(c.backends))
	for n := range c.backends {
		names = append(names, n)
	}
	c.mu.RUnlock()

	status := HealthStatus{
		Details:   make(map[string]string, len(names)),
		CheckedAt: time.Now().UTC(),
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		healthy int
		total   int
	)

	for _, name := range names {
		c.mu.RLock()
		backend, ok := c.backends[name]
		c.mu.RUnlock()

		if !ok {
			continue
		}

		wg.Add(1)
		total++

		go func(n string, b contracts.HealthChecker) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			err := b.Check(checkCtx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				status.Details[n] = "unhealthy: " + err.Error()
			} else {
				status.Details[n] = "ok"
				healthy++
			}
		}(name, backend)
	}

	wg.Wait()

	switch {
	case healthy == total:
		status.Status = "healthy"
	case healthy > 0:
		status.Status = "degraded"
	default:
		status.Status = "unhealthy"
	}

	return status
}

// CheckBackend performs a health probe on a single named backend.
// It returns an error if the backend is not registered or if its
// Check method fails.
func (c *Checker) CheckBackend(ctx context.Context, name string) error {
	c.mu.RLock()
	backend, ok := c.backends[name]
	c.mu.RUnlock()

	if !ok {
		return cacheerrors.Factory.InvalidConfig("Checker.CheckBackend",
			fmt.Sprintf("backend %q is not registered", name))
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return backend.Check(checkCtx)
}

// Backends returns the names of all registered backends.
func (c *Checker) Backends() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.backends))
	for n := range c.backends {
		names = append(names, n)
	}
	return names
}
