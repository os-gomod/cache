// Package redis implements a Redis-backed cache store using go-redis/v9.
// It provides the full Cache interface with support for key prefixing,
// connection pooling, and Lua script-based atomic operations.
package redis

import (
	"time"

	"github.com/os-gomod/cache/v2/internal/middleware"
)

// Option is a functional option for configuring the Redis Store.
type Option func(*config)

// WithAddress sets the Redis server address in the format "host:port".
// Default: "localhost:6379".
func WithAddress(addrs ...string) Option {
	return func(c *config) { c.addresses = addrs }
}

// WithPassword sets the password for Redis AUTH. Default: "" (no auth).
func WithPassword(pwd string) Option {
	return func(c *config) { c.password = pwd }
}

// WithDB sets the Redis database index. Default: 0.
func WithDB(db int) Option {
	return func(c *config) { c.db = db }
}

// WithPoolSize sets the maximum number of socket connections in the pool.
// Default: 10 * GOMAXPROCS.
func WithPoolSize(n int) Option {
	return func(c *config) { c.poolSize = n }
}

// WithKeyPrefix sets a prefix prepended to all keys in Redis.
// This allows multiple cache instances to share the same Redis cluster.
// Default: "" (no prefix).
func WithKeyPrefix(prefix string) Option {
	return func(c *config) { c.keyPrefix = prefix }
}

// WithTTL sets the default time-to-live for entries. Default: 5 minutes.
func WithTTL(d time.Duration) Option {
	return func(c *config) { c.defaultTTL = d }
}

// WithDialTimeout sets the timeout for establishing a new connection.
// Default: 5 seconds.
func WithDialTimeout(d time.Duration) Option {
	return func(c *config) { c.dialTimeout = d }
}

// WithReadTimeout sets the timeout for socket reads. Default: 3 seconds.
func WithReadTimeout(d time.Duration) Option {
	return func(c *config) { c.readTimeout = d }
}

// WithWriteTimeout sets the timeout for socket writes. Default: 3 seconds.
func WithWriteTimeout(d time.Duration) Option {
	return func(c *config) { c.writeTimeout = d }
}

// WithInterceptors sets the observability interceptors for the Redis store.
func WithInterceptors(i ...middleware.Interceptor) Option {
	return func(c *config) { c.interceptors = i }
}

// config holds the validated configuration for the Redis Store.
type config struct {
	addresses    []string
	password     string
	db           int
	poolSize     int
	keyPrefix    string
	defaultTTL   time.Duration
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	interceptors []middleware.Interceptor
}

// defaultConfig returns the default Redis configuration.
func defaultConfig() config {
	return config{
		addresses:    []string{"localhost:6379"},
		password:     "",
		db:           0,
		poolSize:     0, // 0 means use go-redis default
		keyPrefix:    "",
		defaultTTL:   5 * time.Minute,
		dialTimeout:  5 * time.Second,
		readTimeout:  3 * time.Second,
		writeTimeout: 3 * time.Second,
	}
}

// apply applies all options to the default config.
func (c *config) apply(opts ...Option) {
	*c = defaultConfig()
	for _, opt := range opts {
		opt(c)
	}
}
