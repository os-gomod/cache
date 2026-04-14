package manager

import (
	"context"
	"time"

	"github.com/os-gomod/cache/internal/keyutil"
)

// Namespace is a convenience wrapper that prefixes all keys with a fixed
// prefix. It is NOT a backend.Backend — it is a thin delegation layer
// that routes operations through its parent CacheManager.
type Namespace struct {
	manager *CacheManager
	prefix  string
}

// Get retrieves a value, prepending the namespace prefix to the key.
func (n *Namespace) Get(ctx context.Context, key string) ([]byte, error) {
	return n.manager.Get(ctx, n.key(key))
}

// Set stores a value, prepending the namespace prefix to the key.
func (n *Namespace) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return n.manager.Set(ctx, n.key(key), value, ttl)
}

// Delete removes a key, prepending the namespace prefix.
func (n *Namespace) Delete(ctx context.Context, key string) error {
	return n.manager.Delete(ctx, n.key(key))
}

// Prefix returns the namespace prefix.
func (n *Namespace) Prefix() string { return n.prefix }

// key prepends the namespace prefix to the user-provided key.
func (n *Namespace) key(k string) string {
	return keyutil.BuildKey(n.prefix, k)
}
