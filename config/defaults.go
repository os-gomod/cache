// Package config provides configuration types and validation for in-memory,
// Redis, and layered cache backends, as well as environment variable binding.
package config

import "time"

// SetDefaultInt sets the pointed-to int to defaultValue if it is currently zero.
func SetDefaultInt(field *int, defaultValue int) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultInt64 sets the pointed-to int64 to defaultValue if it is currently zero.
func SetDefaultInt64(field *int64, defaultValue int64) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultDuration sets the pointed-to Duration to defaultValue if it is currently zero.
func SetDefaultDuration(field *time.Duration, defaultValue time.Duration) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultString sets the pointed-to string to defaultValue if it is currently empty.
func SetDefaultString(field *string, defaultValue string) {
	if *field == "" {
		*field = defaultValue
	}
}

// SetDefaultBool sets the pointed-to bool to defaultValue if it is currently false.
func SetDefaultBool(field *bool, defaultValue bool) {
	if !*field {
		*field = defaultValue
	}
}

// SetDefaultFloat64 sets the pointed-to float64 to defaultValue if it is currently zero.
func SetDefaultFloat64(field *float64, defaultValue float64) {
	if *field == 0.0 {
		*field = defaultValue
	}
}
