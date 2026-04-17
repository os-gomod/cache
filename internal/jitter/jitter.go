// Package jitter provides randomized delay utilities for retry backoff strategies.
package jitter

import (
	"math/rand"
	"time"
)

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
