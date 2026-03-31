// Package config provides configuration types, defaults and validation for all
// cache backends.
package config

import "time"

// SetDefaultInt sets field to defaultValue when *field == 0.
func SetDefaultInt(field *int, defaultValue int) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultInt64 sets field to defaultValue when *field == 0.
func SetDefaultInt64(field *int64, defaultValue int64) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultDuration sets field to defaultValue when *field == 0.
func SetDefaultDuration(field *time.Duration, defaultValue time.Duration) {
	if *field == 0 {
		*field = defaultValue
	}
}

// SetDefaultString sets field to defaultValue when *field == "".
func SetDefaultString(field *string, defaultValue string) {
	if *field == "" {
		*field = defaultValue
	}
}

// SetDefaultBool sets field to defaultValue when *field == false (zero value).
// When defaultValue is false this is a no-op; use an explicit assignment instead.
func SetDefaultBool(field *bool, defaultValue bool) {
	if field == nil {
		return
	}
	if !*field {
		*field = defaultValue
	}
}

// SetDefaultFloat64 sets field to defaultValue when *field == 0.0.
func SetDefaultFloat64(field *float64, defaultValue float64) {
	if *field == 0.0 {
		*field = defaultValue
	}
}
