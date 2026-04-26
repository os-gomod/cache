package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
	"github.com/os-gomod/cache/v2/internal/lifecycle"
	"github.com/os-gomod/cache/v2/internal/middleware"
)

// Manager manages multiple named cache backends, providing a single
// entry point for multi-backend cache operations. It supports a
// default backend for convenience and allows switching between
// backends by name.
//
// Manager is thread-safe and all methods can be called concurrently.
//
// Example:
//
//	mgr, _ := cache.NewManager(
//	    cache.WithNamedBackend("local", localCache),
//	    cache.WithNamedBackend("remote", redisCache),
//	    cache.WithDefaultBackend("local"),
//	)
//	defer mgr.Close(context.Background())
//
//	// Use default backend
//	mgr.Set(ctx, "key", value, ttl)
//
//	// Use specific backend
//	backend, _ := mgr.Backend("remote")
//	backend.Set(ctx, "key", value, ttl)
type Manager struct {
	backends map[string]Backend
	defaults string // name of the default backend
	mu       sync.RWMutex
	closed   lifecycle.Guard
}

// ManagerOption is a functional option for configuring a Manager.
type ManagerOption func(*Manager)

// WithDefaultBackend sets the named backend as the default. The name
// must match a backend registered with WithNamedBackend, or the
// first registered backend will be used.
func WithDefaultBackend(b Backend) ManagerOption {
	return func(m *Manager) {
		name := "default"
		if n, ok := b.(interface{ Name() string }); ok {
			name = n.Name()
		}
		m.backends[name] = b
		m.defaults = name
	}
}

// WithNamedBackend registers a backend under the given name. The name
// is used to retrieve the backend later with Manager.Backend(name).
func WithNamedBackend(name string, b Backend) ManagerOption {
	return func(m *Manager) {
		m.backends[name] = b
		// Set as default if no default has been set yet.
		if m.defaults == "" {
			m.defaults = name
		}
	}
}

// WithManagerMiddleware applies the given middleware chain to every backend
// registered with the Manager. Middleware is applied in order.
func WithManagerMiddleware(mws ...middleware.Middleware) ManagerOption {
	return func(m *Manager) {
		for name, b := range m.backends {
			m.backends[name] = WithMiddleware(b, mws...)
		}
	}
}

// WithManagerResilience applies the resilience middleware stack (retry,
// circuit breaker, rate limiter) to every backend registered with
// the Manager.
func WithManagerResilience(opts ...ResilienceOption) ManagerOption {
	return func(m *Manager) {
		for name, b := range m.backends {
			m.backends[name] = WithResilience(b, opts...)
		}
	}
}

// NewManager creates a new cache manager with the given options. At
// least one backend must be registered (either via WithDefaultBackend
// or WithNamedBackend).
//
// Returns an error if no backends are registered.
func NewManager(opts ...ManagerOption) (*Manager, error) {
	m := &Manager{
		backends: make(map[string]Backend),
	}
	for _, opt := range opts {
		opt(m)
	}
	if len(m.backends) == 0 {
		return nil, errors.Factory.InvalidConfig("NewManager", "at least one backend required")
	}
	return m, nil
}

// Backend returns the cache backend with the given name.
// Returns errors.NotFound if no backend is registered under that name.
func (m *Manager) Backend(name string) (Backend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.backends[name]
	if !ok {
		return nil, errors.Factory.BackendNotFound(name)
	}
	return b, nil
}

// Default returns the default cache backend. If no default has been
// explicitly set, the first registered backend is returned.
// Returns errors.NotFound if no backends are available.
func (m *Manager) Default() (Backend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.defaults == "" {
		return nil, errors.Factory.InvalidConfig("Manager.Default", "no default backend configured")
	}
	b, ok := m.backends[m.defaults]
	if !ok {
		return nil, errors.Factory.BackendNotFound(m.defaults)
	}
	return b, nil
}

// Get retrieves a value from the default backend.
func (m *Manager) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := m.Default()
	if err != nil {
		return nil, err
	}
	return b.Get(ctx, key)
}

// Set stores a value in the default backend.
func (m *Manager) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	b, err := m.Default()
	if err != nil {
		return err
	}
	return b.Set(ctx, key, value, ttl)
}

// Delete removes a key from the default backend.
func (m *Manager) Delete(ctx context.Context, key string) error {
	b, err := m.Default()
	if err != nil {
		return err
	}
	return b.Delete(ctx, key)
}

// HealthCheck performs a ping on all registered backends and returns
// a map of backend names to their health status. A nil error value
// indicates the backend is healthy.
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]error, len(m.backends))
	for name, b := range m.backends {
		results[name] = b.Ping(ctx)
	}
	return results
}

// Stats returns a snapshot of statistics from all registered backends.
func (m *Manager) Stats() map[string]contracts.StatsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]contracts.StatsSnapshot, len(m.backends))
	for name, b := range m.backends {
		results[name] = b.Stats()
	}
	return results
}

// Close gracefully shuts down all registered backends. Backends that
// are already closed are skipped.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed.Close() {
		return errors.Factory.AlreadyClosed("Manager")
	}

	var errs []error
	for name, b := range m.backends {
		if !b.Closed() {
			if err := b.Close(ctx); err != nil {
				errs = append(errs, fmt.Errorf("backend %q: %w", name, err))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Factory.CloseFailed("Manager", errs[0])
	}
	return nil
}

// Closed returns true if the manager has been closed.
func (m *Manager) Closed() bool {
	return m.closed.IsClosed()
}
