package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// AdaptiveTTL dynamically adjusts time-to-live values based on key
// access patterns. Frequently accessed keys receive longer TTLs to
// maximize cache hit rates, while rarely accessed keys receive shorter
// TTLs to minimize memory waste.
//
// The algorithm uses an exponential weighted moving average (EWMA) of
// inter-access intervals to estimate how "hot" a key is. Hotter keys
// get TTLs closer to maxTTL, while cooler keys get TTLs closer to
// minTTL.
//
// AdaptiveTTL is safe for concurrent use.
//
// Example:
//
//	adaptive := cache.NewAdaptiveTTL(30*time.Second, 10*time.Minute)
//
//	// When setting a cache entry, compute the adaptive TTL:
//	ttl := adaptive.TTL("user:123", 5*time.Minute)
//	backend.Set(ctx, "user:123", data, ttl)
//
//	// When a key is accessed, record it:
//	adaptive.RecordAccess("user:123")
type AdaptiveTTL struct {
	minTTL time.Duration
	maxTTL time.Duration

	// accessMap stores per-key access tracking data.
	accessMap sync.Map // map[string]*accessRecord

	// decayFactor controls how quickly the access score decays.
	// Higher values make the system more responsive to recent changes.
	// Defaults to 0.95.
	decayFactor float64
}

// accessRecord tracks access information for a single key.
type accessRecord struct {
	score      atomic.Int64 // accumulated access score
	lastAccess atomic.Int64 // Unix nanoseconds of last access
}

// NewAdaptiveTTL creates a new adaptive TTL calculator with the given
// minimum and maximum TTL bounds. All computed TTLs will be clamped
// to [minTTL, maxTTL].
//
// Parameters:
//   - min: the minimum TTL for any key (even cold keys get this TTL)
//   - max: the maximum TTL for any key (even hot keys are capped)
//
// Panics if min > max.
func NewAdaptiveTTL(minDur, maxDur time.Duration) *AdaptiveTTL {
	if minDur > maxDur {
		panic("adaptive TTL: min must not exceed max")
	}
	return &AdaptiveTTL{
		minTTL:      minDur,
		maxTTL:      maxDur,
		decayFactor: 0.95,
	}
}

// TTL computes the adaptive TTL for the given key. The baseTTL parameter
// is the application's default TTL for this key type. The computed TTL
// is influenced by how frequently the key has been accessed:
//   - Frequently accessed keys: TTL approaches maxTTL
//   - Rarely accessed keys: TTL approaches minTTL
//   - New keys: returns baseTTL (clamped to [minTTL, maxTTL])
//
// If baseTTL is zero, the midpoint of [minTTL, maxTTL] is used.
func (a *AdaptiveTTL) TTL(key string, baseTTL time.Duration) time.Duration {
	if baseTTL <= 0 {
		baseTTL = (a.minTTL + a.maxTTL) / 2
	}

	val, ok := a.accessMap.Load(key)
	if !ok {
		// Unknown key: return baseTTL clamped to bounds.
		return clampDuration(baseTTL, a.minTTL, a.maxTTL)
	}

	record := val.(*accessRecord)
	score := record.score.Load()

	// Score determines the TTL ratio (0.0 = cold, 1.0 = hot).
	// Normalize score to [0, 1] range using a sigmoid-like function.
	hotness := normalizeScore(score)

	// Compute TTL: minTTL + hotness * (maxTTL - minTTL)
	ttlRange := a.maxTTL - a.minTTL
	computed := a.minTTL + time.Duration(float64(ttlRange)*hotness)

	return clampDuration(computed, a.minTTL, a.maxTTL)
}

// RecordAccess records an access to the given key, updating its
// hotness score. This should be called whenever a key is read from
// the cache (hit or miss). The score decays over time so that
// recently accessed keys have higher scores.
func (a *AdaptiveTTL) RecordAccess(key string) {
	now := time.Now().UnixNano()

	val, _ := a.accessMap.LoadOrStore(key, &accessRecord{
		lastAccess: atomic.Int64{},
		score:      atomic.Int64{},
	})
	record := val.(*accessRecord)

	lastAccess := record.lastAccess.Swap(now)
	currentScore := record.score.Load()

	// Calculate time decay since last access.
	elapsed := now - lastAccess
	decayInterval := int64(5 * time.Second) // decay reference interval

	if elapsed > 0 && decayInterval > 0 {
		// Apply exponential decay based on elapsed time.
		decaySteps := float64(elapsed) / float64(decayInterval)
		decayMultiplier := pow(a.decayFactor, decaySteps)
		currentScore = int64(float64(currentScore) * decayMultiplier)
	}

	// Increment score for this access.
	newScore := currentScore + 100
	record.score.Store(newScore)
	record.lastAccess.Store(now)
}

// Score returns the current hotness score for the given key. Returns 0
// if the key has not been tracked.
func (a *AdaptiveTTL) Score(key string) int64 {
	val, ok := a.accessMap.Load(key)
	if !ok {
		return 0
	}
	return val.(*accessRecord).score.Load()
}

// Reset removes all tracked keys and their scores.
func (a *AdaptiveTTL) Reset() {
	a.accessMap.Range(func(key, _ interface{}) bool {
		a.accessMap.Delete(key)
		return true
	})
}

// Size returns the number of keys currently being tracked.
func (a *AdaptiveTTL) Size() int {
	count := 0
	a.accessMap.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// normalizeScore maps an access score to a [0, 1] hotness value using
// a sigmoid-like function that saturates at high scores.
func normalizeScore(score int64) float64 {
	if score <= 0 {
		return 0.0
	}
	// Sigmoid-like mapping: score -> [0, 1]
	// Using a reference score of 1000 for half-saturation.
	const refScore = 1000.0
	ratio := float64(score) / refScore
	if ratio >= 1.0 {
		return 1.0 - (1.0 / (ratio + 1.0))
	}
	return ratio * (1.0 - ratio/(ratio+1.0))
}

// clampDuration ensures d is within [min, max].
func clampDuration(d, minDur, maxDur time.Duration) time.Duration {
	if d < minDur {
		return minDur
	}
	if d > maxDur {
		return maxDur
	}
	return d
}

// pow computes base^exp for float64 values.
func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1.0
	}
	result := 1.0
	for exp > 1 {
		if int(exp)%2 == 1 {
			result *= base
		}
		base *= base
		exp = float64(int(exp) / 2)
		if exp < 1 {
			break
		}
	}
	return result * base
}
