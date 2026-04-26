package redis

import (
	"strings"
	"testing"
	"time"

	"github.com/os-gomod/cache/v2/internal/keyutil"
)

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{"no prefix", "", "key1", "key1"},
		{"with prefix", "cache:", "key1", "cache:key1"},
		{"empty key", "cache:", "", "cache:"},
		{"long prefix", "myapp:region:cache:", "user:123", "myapp:region:cache:user:123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyutil.BuildKey(tt.prefix, tt.key)
			if got != tt.want {
				t.Errorf("BuildKey(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.want)
			}
		})
	}
}

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{"no prefix", "", "key1", "key1"},
		{"matching prefix", "cache:", "cache:key1", "key1"},
		{"non-matching prefix", "cache:", "key1", "key1"},
		{"empty key", "cache:", "cache:", "cache:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyutil.StripPrefix(tt.prefix, tt.key)
			if got != tt.want {
				t.Errorf("StripPrefix(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.want)
			}
		})
	}
}

func TestOptionsDefaults(t *testing.T) {
	cfg := defaultConfig()
	if cfg.addresses[0] != "localhost:6379" {
		t.Errorf("default address = %q, want %q", cfg.addresses[0], "localhost:6379")
	}
	if cfg.db != 0 {
		t.Errorf("default db = %d, want 0", cfg.db)
	}
	if cfg.defaultTTL != 5*time.Minute {
		t.Errorf("default TTL = %v, want 5m", cfg.defaultTTL)
	}
}

func TestOptionsOverride(t *testing.T) {
	cfg := defaultConfig()
	opts := []Option{
		WithAddress("redis.example.com:6380"),
		WithPassword("secret"),
		WithDB(3),
		WithPoolSize(50),
		WithKeyPrefix("myapp:"),
		WithTTL(10 * time.Minute),
		WithDialTimeout(10 * time.Second),
		WithReadTimeout(5 * time.Second),
		WithWriteTimeout(5 * time.Second),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.addresses[0] != "redis.example.com:6380" {
		t.Errorf("address = %q, want %q", cfg.addresses[0], "redis.example.com:6380")
	}
	if cfg.password != "secret" {
		t.Errorf("password = %q, want %q", cfg.password, "secret")
	}
	if cfg.db != 3 {
		t.Errorf("db = %d, want 3", cfg.db)
	}
	if cfg.poolSize != 50 {
		t.Errorf("poolSize = %d, want 50", cfg.poolSize)
	}
	if cfg.keyPrefix != "myapp:" {
		t.Errorf("keyPrefix = %q, want %q", cfg.keyPrefix, "myapp:")
	}
	if cfg.defaultTTL != 10*time.Minute {
		t.Errorf("defaultTTL = %v, want 10m", cfg.defaultTTL)
	}
}

func TestConfigApply(t *testing.T) {
	cfg := defaultConfig()
	cfg.apply(
		WithAddress("localhost:6379"),
		WithKeyPrefix("test:"),
	)

	if cfg.addresses[0] != "localhost:6379" {
		t.Errorf("address = %q, want %q", cfg.addresses[0], "localhost:6379")
	}
	if !strings.HasPrefix(cfg.keyPrefix, "test:") {
		t.Errorf("keyPrefix = %q, should start with 'test:'", cfg.keyPrefix)
	}
}

func TestStore_Name(t *testing.T) {
	// We can't test New() without a real Redis, but we can verify the
	// Name() method returns the expected value.
	s := &Store{cfg: defaultConfig()}
	if s.Name() != "redis" {
		t.Errorf("Name() = %q, want %q", s.Name(), "redis")
	}
}

func TestLuaScriptsDefined(t *testing.T) {
	if casScript == nil {
		t.Error("casScript should not be nil")
	}
	if getSetScript == nil {
		t.Error("getSetScript should not be nil")
	}
}
