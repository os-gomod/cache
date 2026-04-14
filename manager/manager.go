// Package manager provides CacheManager — the single orchestration point
// for cache backends. Applications should interact with CacheManager rather
// than constructing individual backends directly.
package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/cache/internal/lifecycle"
	"github.com/os-gomod/cache/internal/stats"
	"github.com/os-gomod/cache/observability"
	"github.com/os-gomod/cache/resilience"
)

// Backend is a local interface definition that mirrors cache.Backend.
// Defined locally to avoid circular imports with the root cache package.
type Backend interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error)
	SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	DeleteMulti(ctx context.Context, keys ...string) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
	Stats() stats.Snapshot
	Closed() bool
	Name() string
}

// defaultBackendName is the name used for the default (unnamed) backend.
const defaultBackendName = "default"

// CacheManager is the top-level orchestration layer for cache backends.
// It manages named backends, applies resilience policies and interceptor
// chains, and provides both raw and typed access patterns.
//
// All methods are safe for concurrent use. Backends are wrapped in
// resilience.Cache using the configured Policy before storage.
type CacheManager struct {
	backends     map[string]Backend // named, resilience-wrapped backends
	raw          map[string]Backend // unwrapped originals (for Close)
	policy       resilience.Policy
	chain        *observability.Chain
	interceptors []observability.Interceptor // stored for wrapping backends
	mu           sync.RWMutex
	closed       lifecycle.Guard
	closeOrder   []string // registration order for deterministic close
}

// Option configures a CacheManager.
type Option func(*CacheManager)

// WithBackend registers a named backend. The backend is wrapped in a
// resilience.Cache using the manager's policy. If a backend with the
// same name already exists it is replaced.
func WithBackend(name string, b Backend) Option {
	return func(m *CacheManager) {
		m.registerBackend(name, b)
	}
}

// WithDefaultBackend registers the default backend (name == "default").
func WithDefaultBackend(b Backend) Option {
	return WithBackend(defaultBackendName, b)
}

// WithPolicy sets the resilience policy applied to all backends.
// If not called, NoRetryPolicy is used (direct pass-through).
func WithPolicy(p resilience.Policy) Option {
	return func(m *CacheManager) { m.policy = p }
}

// WithInterceptors sets the observability interceptor chain applied to
// all resilience-wrapped backends.
func WithInterceptors(i ...observability.Interceptor) Option {
	return func(m *CacheManager) {
		m.interceptors = i
		if len(i) > 0 {
			m.chain = observability.NewChain(i...)
		}
	}
}

// New creates a new CacheManager with the given options.
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

// registerBackend wraps b in a resilience.Cache and stores it.
func (m *CacheManager) registerBackend(name string, b Backend) {
	wrapped := resilience.NewCacheWithPolicy(b, m.policy,
		resilience.WithInterceptors(m.interceptors...),
	)
	m.backends[name] = wrapped
	m.raw[name] = b
	// Track insertion order for Close.
	for _, n := range m.closeOrder {
		if n == name {
			return // already present
		}
	}
	m.closeOrder = append(m.closeOrder, name)
}

// ---------------------------------------------------------------------------
// Backend access
// ---------------------------------------------------------------------------

// Backend returns the named backend. If name is empty, the default backend
// is returned. Returns an error if the named backend does not exist.
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

// BackendNames returns the names of all registered backends in
// registration order.
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

// ---------------------------------------------------------------------------
// Core KV — delegated to the default backend
// ---------------------------------------------------------------------------

// Get retrieves a value from the default backend.
func (m *CacheManager) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := m.Backend("")
	if err != nil {
		return nil, err
	}
	return b.Get(ctx, key)
}

// Set stores a value in the default backend.
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

// ---------------------------------------------------------------------------
// Health & Stats
// ---------------------------------------------------------------------------

// HealthCheck runs Ping on all registered backends concurrently. Each Ping
// has a per-backend timeout of min(ctx deadline, 2s). The returned map
// contains backend names that returned errors (nil errors are omitted).
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
			// Per-backend timeout: min(ctx deadline, 2s).
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

// Stats returns a map of backend name to its stats snapshot.
func (m *CacheManager) Stats() map[string]stats.Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]stats.Snapshot, len(m.backends))
	for n, b := range m.backends {
		out[n] = b.Stats()
	}
	return out
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Close closes all backends in registration order, collecting all errors.
// It attempts to close ALL backends even if some return errors. If any
// backend fails to close, a multi-error is returned.
func (m *CacheManager) Close(ctx context.Context) error {
	m.mu.Lock()
	if m.closed.Close() {
		// Already closed.
		m.mu.Unlock()
		return nil
	}
	// Snapshot order and raw backends, then release the lock.
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

// ---------------------------------------------------------------------------
// Namespace
// ---------------------------------------------------------------------------

// Namespace returns a convenience wrapper that prefixes all keys with the
// given prefix. It is NOT a Backend — it is a thin delegation
// layer only.
func (m *CacheManager) Namespace(prefix string) *Namespace {
	return &Namespace{manager: m, prefix: prefix}
}

// ---------------------------------------------------------------------------
// multiError — lightweight multi-error for Close
// ---------------------------------------------------------------------------

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
