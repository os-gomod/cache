package manager

import (
	"context"
	"time"

	"github.com/os-gomod/cache/internal/keyutil"
)

// Namespace provides key-prefixed access to a cache manager's default backend.
type Namespace struct {
	manager *CacheManager
	prefix  string
}

// Get retrieves a value from the default backend using the namespaced key.
func (n *Namespace) Get(ctx context.Context, key string) ([]byte, error) {
	return n.manager.Get(ctx, n.key(key))
}

// Set stores a value on the default backend using the namespaced key.
func (n *Namespace) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return n.manager.Set(ctx, n.key(key), value, ttl)
}

// Delete removes a key from the default backend using the namespaced key.
func (n *Namespace) Delete(ctx context.Context, key string) error {
	return n.manager.Delete(ctx, n.key(key))
}

// Prefix returns the key prefix used by this namespace.
func (n *Namespace) Prefix() string { return n.prefix }

func (n *Namespace) key(k string) string {
	return keyutil.BuildKey(n.prefix, k)
}
