// Package config provides unified configuration for all cache backends with
// sensible defaults, validation, and environment variable overrides. The
// recommended entry point is LoadConfig, which applies defaults, env overrides,
// and validation in a single pipeline.
package config

import "time"

// SetDefaultInt sets *field to defaultValue if it is zero.
func SetDefaultInt(field *int, defaultValue int) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultInt64 sets *field to defaultValue if it is zero.
func SetDefaultInt64(field *int64, defaultValue int64) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultDuration sets *field to defaultValue if it is zero.
func SetDefaultDuration(field *time.Duration, defaultValue time.Duration) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultString sets *field to defaultValue if it is empty.
func SetDefaultString(field *string, defaultValue string) {
	if *field == "" {
		*field = defaultValue
	}
}

// SetDefaultBool sets *field to defaultValue if it is false.
func SetDefaultBool(field *bool, defaultValue bool) {
	if !*field {
		*field = defaultValue
	}
}

// SetDefaultFloat64 sets *field to defaultValue if it is zero.
func SetDefaultFloat64(field *float64, defaultValue float64) {
	if *field == 0.0 {
		*field = defaultValue
	}
}
