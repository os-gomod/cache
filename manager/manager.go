// Package manager provides a CacheManager for managing multiple named cache backends
// with resilience policies and namespace isolation.
package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/cache/internal/backendiface"
	"github.com/os-gomod/cache/internal/lifecycle"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/resilience"
)

// Backend is an alias for the canonical backend interface defined in
// github.com/os-gomod/cache/internal/backendiface.
type Backend = backendiface.Backend

const defaultBackendName = "default"

// CacheManager manages multiple named cache backends with optional resilience
// policies and observability. All backends are automatically wrapped with the
// configured resilience policy on registration.
type CacheManager struct {
	backends     map[string]Backend
	raw          map[string]Backend
	policy       resilience.Policy
	chain        *observability.Chain
	interceptors []observability.Interceptor
	mu           sync.RWMutex
	closed       lifecycle.Guard
	closeOrder   []string
}

// Option is a functional option for CacheManager.
type Option func(*CacheManager)

// WithBackend registers a named cache backend.
func WithBackend(name string, b Backend) Option {
	return func(m *CacheManager) {
		m.registerBackend(name, b)
	}
}

// WithDefaultBackend registers a backend under the default name.
func WithDefaultBackend(b Backend) Option {
	return WithBackend(defaultBackendName, b)
}

// WithPolicy sets the resilience policy applied to all backends.
func WithPolicy(p resilience.Policy) Option {
	return func(m *CacheManager) { m.policy = p }
}

// WithInterceptors sets observability interceptors for all backends.
func WithInterceptors(i ...observability.Interceptor) Option {
	return func(m *CacheManager) {
		m.interceptors = i
		if len(i) > 0 {
			m.chain = observability.NewChain(i...)
		}
	}
}

// New creates a new CacheManager with the given options.
// A default backend must be provided via WithDefaultBackend.
func New(opts ...Option) (*CacheManager, error) {
	m := &CacheManager{
		backends: make(map[string]Backend),
		raw:      make(map[string]Backend),
		policy:   resilience.NoRetryPolicy(),
		chain:    observability.NopChain(),
	}
	for _, opt := range opts {
		opt(m)
	}
	if _, ok := m.backends[defaultBackendName]; !ok {
		return nil, fmt.Errorf("manager: default backend is required; use WithDefaultBackend()")
	}
	return m, nil
}

func (m *CacheManager) registerBackend(name string, b Backend) {
	wrapped := resilience.NewCacheWithPolicy(b, m.policy,
		resilience.WithInterceptors(m.interceptors...),
	)
	m.backends[name] = wrapped
	m.raw[name] = b
	for _, n := range m.closeOrder {
		if n == name {
			return
		}
	}
	m.closeOrder = append(m.closeOrder, name)
}

// Backend returns the named (resilience-wrapped) backend.
// An empty name falls back to the default backend.
func (m *CacheManager) Backend(name string) (Backend, error) {
	if name == "" {
		name = defaultBackendName
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.backends[name]
	if !ok {
		return nil, fmt.Errorf("manager: backend %q not found", name)
	}
	return b, nil
}

// BackendNames returns the names of all registered backends in registration order.
func (m *CacheManager) BackendNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.backends))
	for _, n := range m.closeOrder {
		if _, ok := m.backends[n]; ok {
			names = append(names, n)
		}
	}
	return names
}

// Get retrieves a value from the default backend.
func (m *CacheManager) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := m.Backend("")
	if err != nil {
		return nil, err
	}
	return b.Get(ctx, key)
}

// Set stores a value on the default backend.
func (m *CacheManager) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	b, err := m.Backend("")
	if err != nil {
		return err
	}
	return b.Set(ctx, key, value, ttl)
}

// Delete removes a key from the default backend.
func (m *CacheManager) Delete(ctx context.Context, key string) error {
	b, err := m.Backend("")
	if err != nil {
		return err
	}
	return b.Delete(ctx, key)
}

// HealthCheck pings all backends concurrently and returns per-backend errors.
func (m *CacheManager) HealthCheck(ctx context.Context) map[string]error {
	m.mu.RLock()
	names := make([]string, 0, len(m.backends))
	backends := make(map[string]Backend, len(m.backends))
	for _, n := range m.closeOrder {
		if b, ok := m.backends[n]; ok {
			names = append(names, n)
			backends[n] = b
		}
	}
	m.mu.RUnlock()
	results := make(map[string]error, len(names))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func(n string, b Backend) {
			defer wg.Done()
			timeout := 2 * time.Second
			if dl, ok := ctx.Deadline(); ok {
				if d := time.Until(dl); d < timeout {
					timeout = d
				}
			}
			bctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			err := b.Ping(bctx)
			if err != nil {
				mu.Lock()
				results[n] = err
				mu.Unlock()
			}
		}(name, backends[name])
	}
	wg.Wait()
	return results
}

// Stats returns statistics snapshots for all backends.
func (m *CacheManager) Stats() map[string]stats.Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]stats.Snapshot, len(m.backends))
	for n, b := range m.backends {
		out[n] = b.Stats()
	}
	return out
}

// Close shuts down all backends in reverse registration order.
func (m *CacheManager) Close(ctx context.Context) error {
	m.mu.Lock()
	if m.closed.Close() {
		m.mu.Unlock()
		return nil
	}
	order := make([]string, len(m.closeOrder))
	copy(order, m.closeOrder)
	raws := make(map[string]Backend, len(m.raw))
	for k, v := range m.raw {
		raws[k] = v
	}
	m.mu.Unlock()
	var errs []error
	for _, name := range order {
		if b, ok := raws[name]; ok {
			if err := b.Close(ctx); err != nil {
				errs = append(errs, fmt.Errorf("backend %q: %w", name, err))
			}
		}
	}
	if len(errs) == 1 {
		return errs[0]
	}
	if len(errs) > 1 {
		return &multiError{errs: errs}
	}
	return nil
}

// Closed reports whether the manager has been closed.
func (m *CacheManager) Closed() bool {
	return m.closed.IsClosed()
}

// Namespace creates a new Namespace with the given key prefix.
func (m *CacheManager) Namespace(prefix string) *Namespace {
	return &Namespace{manager: m, prefix: prefix}
}

// multiError combines multiple errors into one.
type multiError struct {
	errs []error
}

func (me *multiError) Error() string {
	s := "manager: multiple errors on close:"
	for _, e := range me.errs {
		s += "\n  - " + e.Error()
	}
	return s
}

func (me *multiError) Unwrap() []error { return me.errs }
