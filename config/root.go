package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the top-level configuration for all cache backends.
type Config struct {
	Memory        *Memory          `config:"memory"`
	Redis         *Redis           `config:"redis"`
	Layered       *Layered         `config:"layered"`
	Resilience    ResilienceConfig `config:"resilience"`
	Observability ObsConfig        `config:"observability"`
}

// ResilienceConfig holds circuit breaker and retry settings.
type ResilienceConfig struct {
	CircuitBreakerThreshold int           `config:"cb_threshold" default:"5"`
	CircuitBreakerTimeout   time.Duration `config:"cb_timeout"   default:"30s"`
	MaxRetries              int           `config:"max_retries"  default:"3"`
}

// ObsConfig holds observability settings for metrics and tracing.
type ObsConfig struct {
	MetricsEnabled bool          `config:"metrics_enabled" default:"false"`
	TracingEnabled bool          `config:"tracing_enabled" default:"false"`
	LogLevel       string        `config:"log_level"       default:"info"`
	SlowThreshold  time.Duration `config:"slow_threshold"  default:"10ms"`
}

// SetDefaults fills zero-valued fields with sensible defaults.
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

// Validate checks the entire configuration for correctness.
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
type (
	ConfigOption func(*loadOpts)
	loadOpts     struct {
		fromEnv  bool
		fromMap  map[string]string
		defaults bool
	}
)

// FromEnv enables environment variable binding during config loading.
func FromEnv() ConfigOption {
	return func(o *loadOpts) { o.fromEnv = true }
}

// FromMap enables map-based config overrides during loading.
func FromMap(m map[string]string) ConfigOption {
	return func(o *loadOpts) { o.fromMap = m }
}

// WithDefaults enables applying default values during config loading.
func WithDefaults() ConfigOption {
	return func(o *loadOpts) { o.defaults = true }
}

// LoadConfig creates and validates a Config using the given options.
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

func setMapValues(cfg *Config, m map[string]string) error {
	var restore []string
	for k, v := range m {
		envKey := "CACHE_" + strings.ToUpper(k)
		prev, wasSet := os.LookupEnv(envKey)
		os.Setenv(envKey, v)
		if wasSet {
			restore = append(restore, envKey+"\x00"+prev)
		} else {
			restore = append(restore, envKey)
		}
	}
	err := ApplyEnv(cfg)
	for _, entry := range restore {
		if idx := strings.IndexByte(entry, '\x00'); idx >= 0 {
			os.Setenv(
				entry[:idx],
				entry[idx+1:],
			)
		} else {
			_ = os.Unsetenv(entry)
		}
	}
	return err
}
