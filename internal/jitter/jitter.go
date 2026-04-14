// Package jitter provides a simple function to add random jitter to durations, which can help prevent thundering herd
// problems when many cache entries expire at the same time.
package jitter

import (
	"math/rand"
	"time"
)

// AddJitter adds ±15% jitter by default to prevent thundering herd on mass expiry.
func AddJitter(d time.Duration, factor float64) time.Duration {
	if d == 0 || factor <= 0 {
		return d
	}
	if factor > 0.5 {
		factor = 0.5
	}
	j := time.Duration(float64(d) * factor * (rand.Float64()*2 - 1))
	return d + j
}
