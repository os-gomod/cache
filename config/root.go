package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the top-level configuration for the cache library. It aggregates
// all backend configs (Memory, Redis, Layered) and cross-cutting configs
// (Resilience, Observability) into a single entry point.
//
// Use LoadConfig to create an initialized and validated Config:
//
//	cfg, err := config.LoadConfig(
//	    config.WithDefaults(),
//	    config.FromEnv(),
//	)
type Config struct {
	Memory        *Memory          `config:"memory"`
	Redis         *Redis           `config:"redis"`
	Layered       *Layered         `config:"layered"`
	Resilience    ResilienceConfig `config:"resilience"`
	Observability ObsConfig        `config:"observability"`
}

// ResilienceConfig controls the resilience behavior applied across all
// cache backends managed by a CacheManager. These settings are used by the
// resilience package to configure circuit breakers, retries, and timeouts.
type ResilienceConfig struct {
	CircuitBreakerThreshold int           `config:"cb_threshold" default:"5"`
	CircuitBreakerTimeout   time.Duration `config:"cb_timeout"   default:"30s"`
	MaxRetries              int           `config:"max_retries"  default:"3"`
}

// ObsConfig controls the observability behavior applied across all cache
// backends. These settings configure metrics collection, distributed tracing,
// logging verbosity, and slow-operation detection.
type ObsConfig struct {
	MetricsEnabled bool          `config:"metrics_enabled" default:"false"`
	TracingEnabled bool          `config:"tracing_enabled" default:"false"`
	LogLevel       string        `config:"log_level"       default:"info"`
	SlowThreshold  time.Duration `config:"slow_threshold"  default:"10ms"`
}

// SetDefaults populates zero-valued fields with their default values.
func (c *Config) SetDefaults() {
	if c.Memory == nil {
		c.Memory = DefaultMemory()
	}
	if c.Redis == nil {
		c.Redis = DefaultRedis()
	}
	if c.Layered == nil {
		c.Layered = DefaultLayered()
	}
	SetDefaultInt(&c.Resilience.CircuitBreakerThreshold, 5)
	SetDefaultDuration(&c.Resilience.CircuitBreakerTimeout, 30*time.Second)
	SetDefaultInt(&c.Resilience.MaxRetries, 3)
	SetDefaultBool(&c.Observability.MetricsEnabled, false)
	SetDefaultBool(&c.Observability.TracingEnabled, false)
	SetDefaultString(&c.Observability.LogLevel, "info")
	SetDefaultDuration(&c.Observability.SlowThreshold, 10*time.Millisecond)
}

// Validate checks the entire Config for consistency and returns the first
// error encountered. Each sub-config is validated independently; errors from
// multiple sub-configs are not aggregated at this level.
func (c *Config) Validate() error {
	if c.Memory != nil {
		if err := c.Memory.Validate(); err != nil {
			return fmt.Errorf("memory config: %w", err)
		}
	}
	if c.Redis != nil {
		if err := c.Redis.Validate(); err != nil {
			return fmt.Errorf("redis config: %w", err)
		}
	}
	if c.Layered != nil {
		if err := c.Layered.Validate(); err != nil {
			return fmt.Errorf("layered config: %w", err)
		}
	}
	if c.Resilience.CircuitBreakerThreshold < 0 {
		return fmt.Errorf("resilience: circuit breaker threshold must be >= 0, got %d",
			c.Resilience.CircuitBreakerThreshold)
	}
	if c.Resilience.CircuitBreakerTimeout < 0 {
		return fmt.Errorf("resilience: circuit breaker timeout must be >= 0, got %v",
			c.Resilience.CircuitBreakerTimeout)
	}
	if c.Resilience.MaxRetries < 0 {
		return fmt.Errorf("resilience: max retries must be >= 0, got %d",
			c.Resilience.MaxRetries)
	}
	switch c.Observability.LogLevel {
	case "debug", "info", "warn", "error", "":
		// valid (empty is allowed when defaults haven't been applied)
	default:
		return fmt.Errorf("observability: log level must be one of debug/info/warn/error, got %q",
			c.Observability.LogLevel)
	}
	if c.Observability.SlowThreshold < 0 {
		return fmt.Errorf("observability: slow threshold must be >= 0, got %v",
			c.Observability.SlowThreshold)
	}
	return nil
}

// ConfigOption is a functional option for LoadConfig.
type ConfigOption func(*loadOpts)

type loadOpts struct {
	fromEnv  bool
	fromMap  map[string]string
	defaults bool
}

// FromEnv enables environment variable overrides (CACHE_* variables) during
// config loading. ApplyEnv is called after defaults are set but before
// validation, so env vars take precedence over defaults.
func FromEnv() ConfigOption {
	return func(o *loadOpts) { o.fromEnv = true }
}

// FromMap overrides config values from a string map. The map keys should
// match the config tag names (e.g., "cb_threshold", "log_level"). This is
// useful for loading config from YAML, JSON, or other structured sources.
// Map values are applied after defaults and before validation.
func FromMap(m map[string]string) ConfigOption {
	return func(o *loadOpts) { o.fromMap = m }
}

// WithDefaults populates zero-valued fields with sensible defaults. When
// this option is not provided, sub-configs (Memory, Redis, Layered) remain
// nil and only the top-level Resilience and Observability sections get
// default values.
func WithDefaults() ConfigOption {
	return func(o *loadOpts) { o.defaults = true }
}

// LoadConfig creates and validates a Config using the provided options.
// The load pipeline is:
//  1. Create an empty Config
//  2. If WithDefaults: call SetDefaults (populates all sub-configs)
//  3. If FromMap: apply map overrides via setMapValues
//  4. If FromEnv: apply environment variable overrides via ApplyEnv
//  5. Validate the final config
//
// This is the single recommended entry point for creating a Config.
func LoadConfig(opts ...ConfigOption) (*Config, error) {
	var lo loadOpts
	for _, opt := range opts {
		opt(&lo)
	}

	cfg := &Config{}

	if lo.defaults {
		cfg.SetDefaults()
	}

	if lo.fromMap != nil {
		if err := setMapValues(cfg, lo.fromMap); err != nil {
			return nil, fmt.Errorf("config: map override: %w", err)
		}
	}

	if lo.fromEnv {
		if err := ApplyEnv(cfg); err != nil {
			return nil, fmt.Errorf("config: env override: %w", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// setMapValues overrides config fields from a string map. The map keys
// correspond to the config:"<name>" struct tags. It temporarily sets
// environment variables and uses ApplyEnv's parsing logic to avoid
// duplicating the type-parsing code.
func setMapValues(cfg *Config, m map[string]string) error {
	// Set env vars temporarily, then restore. This reuses ApplyEnv's
	// parsing logic so we don't duplicate type-conversion code.
	var restore []string
	for k, v := range m {
		envKey := "CACHE_" + strings.ToUpper(k)
		prev, wasSet := os.LookupEnv(envKey)
		os.Setenv(envKey, v) //nolint:errcheck,revive // intentional env mutation for config loading
		if wasSet {
			restore = append(restore, envKey+"\x00"+prev)
		} else {
			restore = append(restore, envKey)
		}
	}

	err := ApplyEnv(cfg)

	// Restore env to previous state.
	for _, entry := range restore {
		if idx := strings.IndexByte(entry, '\x00'); idx >= 0 {
			os.Setenv(
				entry[:idx],
				entry[idx+1:],
			) //nolint:errcheck,revive // restoring previous value
		} else {
			_ = os.Unsetenv(entry)
		}
	}

	return err
}
