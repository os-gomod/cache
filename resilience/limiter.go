package resilience

import (
	"context"
	"sync"
	"time"
)

type tokenBucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newTokenBucket(rps float64, burst int) *tokenBucket {
	b := float64(burst)
	if b < 0 {
		b = 0
	}
	return &tokenBucket{rate: rps, burst: b, tokens: b, last: time.Now()}
}

func (b *tokenBucket) allow(now time.Time) bool {
	if b == nil || b.rate <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = min(b.burst, b.tokens+elapsed*b.rate)
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// LimiterConfig holds separate read and write rate limiter settings.
type LimiterConfig struct {
	ReadRPS    float64
	ReadBurst  int
	WriteRPS   float64
	WriteBurst int
}

// Limiter provides token-bucket rate limiting for read and write operations.
type Limiter struct {
	read  *tokenBucket
	write *tokenBucket
}

// NewLimiter creates a Limiter with the same rate and burst for both reads and writes.
func NewLimiter(rps float64, burst int) *Limiter {
	return NewLimiterWithConfig(LimiterConfig{
		ReadRPS: rps, ReadBurst: burst,
		WriteRPS: rps, WriteBurst: burst,
	})
}

// NewLimiterWithConfig creates a Limiter with separate read and write settings.
func NewLimiterWithConfig(cfg LimiterConfig) *Limiter {
	return &Limiter{
		read:  newTokenBucket(cfg.ReadRPS, cfg.ReadBurst),
		write: newTokenBucket(cfg.WriteRPS, cfg.WriteBurst),
	}
}

// AllowRead reports whether a read operation should be permitted by the rate limiter.
func (l *Limiter) AllowRead(ctx context.Context) bool {
	if l == nil || ctx.Err() != nil {
		return l == nil
	}
	return l.read.allow(time.Now())
}

// AllowWrite reports whether a write operation should be permitted by the rate limiter.
func (l *Limiter) AllowWrite(ctx context.Context) bool {
	if l == nil || ctx.Err() != nil {
		return l == nil
	}
	return l.write.allow(time.Now())
}
