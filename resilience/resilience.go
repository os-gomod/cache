// Package resilience provides a protective wrapper for any cache backend with
// circuit breaking, token-bucket rate limiting, and pluggable observability hooks.
package resilience

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	_errors "github.com/os-gomod/cache/errors"
)

// ----------------------------------------------------------------------------
// Hooks — pluggable observability callbacks
// ----------------------------------------------------------------------------

// Hooks defines optional callbacks invoked around cache operations.
// All fields are optional; a nil function is never called.
type Hooks struct {
	// OnGet is called after every Get attempt.
	// hit is true only for successful reads; errKind is "" for a clean miss,
	// "error" for a transport failure.
	OnGet func(ctx context.Context, key string, hit bool, errKind string, d time.Duration)

	// OnSet is called after a successful Set.
	OnSet func(ctx context.Context, key string, size int, d time.Duration)

	// OnError is called whenever a backend error occurs (not for rate-limit or
	// circuit-open rejections; those surface via sentinel errors).
	OnError func(ctx context.Context, op string, err error)

	// OnStateChange is called when the circuit breaker changes state.
	OnStateChange func(from, to State)
}

func (h *Hooks) onGet(ctx context.Context, key string, hit bool, kind string, d time.Duration) {
	if h != nil && h.OnGet != nil {
		h.OnGet(ctx, key, hit, kind, d)
	}
}

func (h *Hooks) onSet(ctx context.Context, key string, size int, d time.Duration) {
	if h != nil && h.OnSet != nil {
		h.OnSet(ctx, key, size, d)
	}
}

func (h *Hooks) onError(ctx context.Context, op string, err error) {
	if h != nil && h.OnError != nil {
		h.OnError(ctx, op, err)
	}
}

func (h *Hooks) onStateChange(from, to State) {
	if h != nil && h.OnStateChange != nil {
		h.OnStateChange(from, to)
	}
}

// ----------------------------------------------------------------------------
// Circuit breaker
// ----------------------------------------------------------------------------

// State is the circuit-breaker state.
type State int32

const (
	// StateClosed allows all requests through normally.
	StateClosed State = iota
	// StateOpen rejects all requests until the reset timeout elapses.
	StateOpen
	// StateHalfOpen admits exactly one trial request to test recovery.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// circuitState is the shared mutable state of the breaker, stored behind an
// atomic.Pointer so the entire snapshot can be read lock-free.
//
// Cache-line padding separates the hot write field (failures) from the hot
// read field (state) to prevent false sharing on multi-core systems.
type circuitState struct {
	failures    int64
	_pad0       [56]byte //nolint:unused // cache-line pad
	state       int32
	lastFailure int64
	_pad1       [52]byte //nolint:unused // cache-line pad
}

// CircuitBreaker protects a backend by blocking requests after repeated
// failures and resuming them after a recovery window.
//
// State machine:
//
//	Closed ──(threshold failures)──► Open ──(resetTimeout)──► HalfOpen
//	  ▲                                                            │
//	  └────────────────────(success)──────────────────────────────┘
//	                   HalfOpen ──(failure)──► Open
type CircuitBreaker struct {
	st            atomic.Pointer[circuitState]
	failureThresh int64
	resetTimeout  time.Duration
	hooks         *Hooks
}

// NewCircuitBreaker creates a circuit breaker.
//
//   - threshold: consecutive failures before opening (clamped to ≥ 1).
//   - resetTimeout: how long to stay open before admitting a trial request.
//     0 means the breaker never auto-recovers (manual Reset required).
func NewCircuitBreaker(threshold int64, resetTimeout time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 1
	}
	if resetTimeout < 0 {
		resetTimeout = 0
	}
	cb := &CircuitBreaker{failureThresh: threshold, resetTimeout: resetTimeout}
	cb.st.Store(&circuitState{state: int32(StateClosed)})
	return cb
}

func (cb *CircuitBreaker) withHooks(h *Hooks) { cb.hooks = h }

// State returns the current circuit state.
func (cb *CircuitBreaker) State() State {
	if cb == nil {
		return StateClosed
	}
	return State(atomic.LoadInt32(&cb.st.Load().state))
}

// Allow reports whether the caller may proceed.
// Half-open: exactly one goroutine wins the CAS to StateHalfOpen; all others
// are rejected, preventing a thundering herd of trial requests.
func (cb *CircuitBreaker) Allow() bool {
	if cb == nil {
		return true
	}
	st := cb.st.Load()
	switch State(atomic.LoadInt32(&st.state)) {
	case StateClosed:
		return true
	case StateOpen:
		if cb.resetTimeout == 0 {
			return false
		}
		elapsed := time.Duration(time.Now().UnixNano() - atomic.LoadInt64(&st.lastFailure))
		if elapsed < cb.resetTimeout {
			return false
		}
		return atomic.CompareAndSwapInt32(&st.state, int32(StateOpen), int32(StateHalfOpen))
	case StateHalfOpen:
		return false
	default:
		return false
	}
}

// Success records a successful backend response.
func (cb *CircuitBreaker) Success() {
	if cb == nil {
		return
	}
	st := cb.st.Load()
	prev := State(atomic.LoadInt32(&st.state))
	atomic.StoreInt64(&st.failures, 0)
	if prev != StateClosed {
		atomic.StoreInt32(&st.state, int32(StateClosed))
		cb.hooks.onStateChange(prev, StateClosed)
	}
}

// Failure records a failed backend response.
func (cb *CircuitBreaker) Failure() {
	if cb == nil {
		return
	}
	st := cb.st.Load()
	atomic.StoreInt64(&st.lastFailure, time.Now().UnixNano())
	failures := atomic.AddInt64(&st.failures, 1)
	prev := State(atomic.LoadInt32(&st.state))
	if prev == StateHalfOpen || failures >= cb.failureThresh {
		if atomic.CompareAndSwapInt32(&st.state, int32(prev), int32(StateOpen)) {
			cb.hooks.onStateChange(prev, StateOpen)
		}
	}
}

// Reset forces the breaker back to closed with a zeroed failure count.
func (cb *CircuitBreaker) Reset() {
	if cb == nil {
		return
	}
	st := cb.st.Load()
	prev := State(atomic.LoadInt32(&st.state))
	atomic.StoreInt64(&st.failures, 0)
	atomic.StoreInt32(&st.state, int32(StateClosed))
	if prev != StateClosed {
		cb.hooks.onStateChange(prev, StateClosed)
	}
}

// ----------------------------------------------------------------------------
// Token-bucket rate limiter
// ----------------------------------------------------------------------------

type tokenBucket struct {
	mu     sync.Mutex
	rate   float64 // tokens/second; 0 = unlimited
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

// LimiterConfig holds independent rate limits for reads and writes.
type LimiterConfig struct {
	ReadRPS    float64
	ReadBurst  int
	WriteRPS   float64
	WriteBurst int
}

// Limiter provides independent read/write token-bucket rate limiting.
type Limiter struct {
	read  *tokenBucket
	write *tokenBucket
}

// NewLimiter creates a Limiter with the same RPS and burst for both reads
// and writes.
func NewLimiter(rps float64, burst int) *Limiter {
	return NewLimiterWithConfig(LimiterConfig{
		ReadRPS: rps, ReadBurst: burst,
		WriteRPS: rps, WriteBurst: burst,
	})
}

// NewLimiterWithConfig creates a Limiter with independent read/write limits.
func NewLimiterWithConfig(cfg LimiterConfig) *Limiter {
	return &Limiter{
		read:  newTokenBucket(cfg.ReadRPS, cfg.ReadBurst),
		write: newTokenBucket(cfg.WriteRPS, cfg.WriteBurst),
	}
}

// AllowRead reports whether a read operation may proceed.
func (l *Limiter) AllowRead(ctx context.Context) bool {
	if l == nil || ctx.Err() != nil {
		return l == nil
	}
	return l.read.allow(time.Now())
}

// AllowWrite reports whether a write operation may proceed.
func (l *Limiter) AllowWrite(ctx context.Context) bool {
	if l == nil || ctx.Err() != nil {
		return l == nil
	}
	return l.write.allow(time.Now())
}

// ----------------------------------------------------------------------------
// Backend interface
// ----------------------------------------------------------------------------

// Backend is the minimum cache API required by the resilience wrapper.
// It matches CoreCache from the root package so Cache is a drop-in replacement.
type Backend interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
}

type closeBackend interface {
	Close(ctx context.Context) error
}

// ----------------------------------------------------------------------------
// Options and Cache wrapper
// ----------------------------------------------------------------------------

// Options configures the resilience wrapper.
type Options struct {
	// CircuitBreaker guards against repeated backend failures (nil = disabled).
	CircuitBreaker *CircuitBreaker

	// Limiter enforces read/write rate limits (nil = disabled).
	Limiter *Limiter

	// Hooks provides observability callbacks (nil = disabled).
	Hooks *Hooks
}

// Cache wraps a Backend with circuit breaking, rate limiting, and hooks.
type Cache struct {
	backend Backend
	cb      *CircuitBreaker
	limiter *Limiter
	hooks   *Hooks
}

// NewCache creates a resilient cache wrapper.
func NewCache(b Backend, opts Options) *Cache {
	c := &Cache{
		backend: b,
		cb:      opts.CircuitBreaker,
		limiter: opts.Limiter,
		hooks:   opts.Hooks,
	}
	if c.cb != nil {
		c.cb.withHooks(c.hooks)
	}
	return c
}

// guard runs rate-limit and circuit-breaker checks before an operation.
func (c *Cache) guard(ctx context.Context, allowFn func(context.Context) bool, op string) error {
	if c.limiter != nil && !allowFn(ctx) {
		c.hooks.onError(ctx, op+"_rate_limited", _errors.ErrRateLimited)
		return _errors.ErrRateLimited
	}
	if c.cb != nil && !c.cb.Allow() {
		c.hooks.onError(ctx, op+"_circuit_open", _errors.ErrCircuitOpen)
		return _errors.ErrCircuitOpen
	}
	return nil
}

// record updates the circuit breaker and fires the error hook on failure.
func (c *Cache) record(ctx context.Context, op string, err error) {
	if err != nil {
		if c.cb != nil {
			c.cb.Failure()
		}
		c.hooks.onError(ctx, op, err)
		return
	}
	if c.cb != nil {
		c.cb.Success()
	}
}

// Get retrieves a value, subject to rate limiting and circuit breaking.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()
	if err := c.guard(ctx, c.limiter.AllowRead, "get"); err != nil {
		return nil, err
	}
	val, err := c.backend.Get(ctx, key)
	c.record(ctx, "get", err)
	errKind := ""
	if err != nil {
		errKind = "error"
	}
	c.hooks.onGet(ctx, key, err == nil, errKind, time.Since(start))
	return val, err
}

// Set stores a value, subject to rate limiting and circuit breaking.
func (c *Cache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	start := time.Now()
	if err := c.guard(ctx, c.limiter.AllowWrite, "set"); err != nil {
		return err
	}
	err := c.backend.Set(ctx, key, val, ttl)
	c.record(ctx, "set", err)
	if err == nil {
		c.hooks.onSet(ctx, key, len(val), time.Since(start))
	}
	return err
}

// Delete removes a key, subject to rate limiting and circuit breaking.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.guard(ctx, c.limiter.AllowWrite, "delete"); err != nil {
		return err
	}
	err := c.backend.Delete(ctx, key)
	c.record(ctx, "delete", err)
	return err
}

// Exists checks key presence, subject to rate limiting and circuit breaking.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.guard(ctx, c.limiter.AllowRead, "exists"); err != nil {
		return false, err
	}
	ok, err := c.backend.Exists(ctx, key)
	c.record(ctx, "exists", err)
	return ok, err
}

// TTL returns the remaining TTL, subject to rate limiting and circuit breaking.
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.guard(ctx, c.limiter.AllowRead, "ttl"); err != nil {
		return 0, err
	}
	d, err := c.backend.TTL(ctx, key)
	c.record(ctx, "ttl", err)
	return d, err
}

// Close closes the wrapped backend if it implements the closer interface.
func (c *Cache) Close(ctx context.Context) error {
	if c == nil || c.backend == nil {
		return nil
	}
	if closer, ok := c.backend.(closeBackend); ok {
		return closer.Close(ctx)
	}
	return nil
}
