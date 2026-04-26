package cache

import (
	"context"
	"time"

	"github.com/os-gomod/cache/v2/internal/contracts"
	"github.com/os-gomod/cache/v2/internal/errors"
)

// Namespace provides key-prefixed isolation within a cache backend.
// All operations automatically prepend the configured prefix to keys,
// allowing multiple logical caches to share a single physical backend
// without key collisions.
//
// Namespace implements the Backend interface, so it can be used anywhere
// a Backend is expected. It can also be nested to create hierarchical
// namespaces (e.g., "app:user:sessions").
//
// Example:
//
//	ns := cache.NewNamespace("myapp:sessions", backend)
//	ns.Set(ctx, "abc123", sessionData, 30*time.Minute)
//	// Actually stores key "myapp:sessions:abc123"
type Namespace struct {
	prefix  string
	backend Backend
}

// NewNamespace creates a new Namespace wrapper that prepends the given
// prefix (followed by a colon) to all keys. The prefix must be non-empty.
//
// Returns an error if prefix is empty.
func NewNamespace(prefix string, backend Backend) (*Namespace, error) {
	if prefix == "" {
		return nil, errors.Factory.InvalidConfig("NewNamespace", "namespace prefix must be non-empty")
	}
	return &Namespace{
		prefix:  prefix,
		backend: backend,
	}, nil
}

// prefixedKey returns the full key with the namespace prefix prepended.
func (n *Namespace) prefixedKey(key string) string {
	return n.prefix + ":" + key
}

// Get retrieves the value for the given key from the namespace.
func (n *Namespace) Get(ctx context.Context, key string) ([]byte, error) {
	return n.backend.Get(ctx, n.prefixedKey(key))
}

// GetMulti retrieves multiple values from the namespace.
func (n *Namespace) GetMulti(ctx context.Context, keys ...string) (map[string][]byte, error) {
	prefixed := make([]string, len(keys))
	for i, k := range keys {
		prefixed[i] = n.prefixedKey(k)
	}
	dataMap, err := n.backend.GetMulti(ctx, prefixed...)
	if err != nil {
		return nil, err
	}
	// Strip prefix from returned keys.
	result := make(map[string][]byte, len(dataMap))
	prefixLen := len(n.prefix) + 1
	for k, v := range dataMap {
		if len(k) > prefixLen {
			result[k[prefixLen:]] = v
		}
	}
	return result, nil
}

// Exists checks whether the key exists in the namespace.
func (n *Namespace) Exists(ctx context.Context, key string) (bool, error) {
	return n.backend.Exists(ctx, n.prefixedKey(key))
}

// TTL returns the remaining time-to-live for the given key.
func (n *Namespace) TTL(ctx context.Context, key string) (time.Duration, error) {
	return n.backend.TTL(ctx, n.prefixedKey(key))
}

// Set stores the value under the namespaced key.
func (n *Namespace) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return n.backend.Set(ctx, n.prefixedKey(key), value, ttl)
}

// SetMulti stores multiple values in the namespace.
func (n *Namespace) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	prefixed := make(map[string][]byte, len(items))
	for k, v := range items {
		prefixed[n.prefixedKey(k)] = v
	}
	return n.backend.SetMulti(ctx, prefixed, ttl)
}

// Delete removes the key from the namespace.
func (n *Namespace) Delete(ctx context.Context, key string) error {
	return n.backend.Delete(ctx, n.prefixedKey(key))
}

// DeleteMulti removes multiple keys from the namespace.
func (n *Namespace) DeleteMulti(ctx context.Context, keys ...string) error {
	prefixed := make([]string, len(keys))
	for i, k := range keys {
		prefixed[i] = n.prefixedKey(k)
	}
	return n.backend.DeleteMulti(ctx, prefixed...)
}

// CompareAndSwap atomically compares and swaps within the namespace.
func (n *Namespace) CompareAndSwap(ctx context.Context, key string, oldValue, newValue []byte, ttl time.Duration) (bool, error) {
	return n.backend.CompareAndSwap(ctx, n.prefixedKey(key), oldValue, newValue, ttl)
}

// SetNX sets the key only if it does not exist, within the namespace.
func (n *Namespace) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	return n.backend.SetNX(ctx, n.prefixedKey(key), value, ttl)
}

// Increment atomically increments a numeric value within the namespace.
func (n *Namespace) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	return n.backend.Increment(ctx, n.prefixedKey(key), delta)
}

// Decrement atomically decrements a numeric value within the namespace.
func (n *Namespace) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return n.backend.Decrement(ctx, n.prefixedKey(key), delta)
}

// GetSet atomically sets a value and returns the previous value within
// the namespace.
func (n *Namespace) GetSet(ctx context.Context, key string, value []byte, ttl time.Duration) ([]byte, error) {
	return n.backend.GetSet(ctx, n.prefixedKey(key), value, ttl)
}

// Keys returns all keys matching the pattern within the namespace.
// The pattern is matched against the original (non-prefixed) keys.
func (n *Namespace) Keys(ctx context.Context, pattern string) ([]string, error) {
	// Prepend prefix to the pattern for backend matching.
	backendPattern := n.prefix + ":" + pattern
	keys, err := n.backend.Keys(ctx, backendPattern)
	if err != nil {
		return nil, err
	}
	// Strip prefix from returned keys.
	prefixLen := len(n.prefix) + 1
	result := make([]string, 0, len(keys))
	for _, k := range keys {
		if len(k) > prefixLen {
			result = append(result, k[prefixLen:])
		}
	}
	return result, nil
}

// Clear removes all entries in the namespace. This only clears entries
// with the configured prefix.
func (n *Namespace) Clear(ctx context.Context) error {
	keys, err := n.Keys(ctx, "*")
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return n.DeleteMulti(ctx, keys...)
}

// Size returns the approximate number of entries in the namespace.
// Note: this scans all keys with the namespace prefix and may be
// expensive for large namespaces.
func (n *Namespace) Size(ctx context.Context) (int64, error) {
	keys, err := n.Keys(ctx, "*")
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// Ping checks connectivity to the underlying backend.
func (n *Namespace) Ping(ctx context.Context) error {
	return n.backend.Ping(ctx)
}

// Close closes the underlying backend.
func (n *Namespace) Close(ctx context.Context) error {
	return n.backend.Close(ctx)
}

// Closed returns true if the underlying backend is closed.
func (n *Namespace) Closed() bool {
	return n.backend.Closed()
}

// Name returns a descriptive name for the namespace.
func (n *Namespace) Name() string {
	return "namespace:" + n.prefix
}

// Stats returns statistics from the underlying backend.
func (n *Namespace) Stats() contracts.StatsSnapshot {
	return n.backend.Stats()
}

// Backend returns the underlying Backend instance.
func (n *Namespace) Backend() Backend {
	return n.backend
}

// Prefix returns the configured namespace prefix.
func (n *Namespace) Prefix() string {
	return n.prefix
}

// Ensure Namespace satisfies the Backend interface at compile time.
var _ Backend = (*Namespace)(nil) //nolint:errcheck // compile-time interface satisfaction
