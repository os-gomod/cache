package stampede

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// DistributedLock represents a distributed lock backed by Redis. It supports
// automatic renewal: once acquired, a background goroutine periodically extends
// the lock's TTL so that long-running computations do not lose their lock
// prematurely.
//
// Callers must always call Release when done with the lock, even if the
// acquisition context was cancelled. Release is idempotent and safe to call
// multiple times.
type DistributedLock struct {
	client   goredis.UniversalClient
	key      string
	token    string
	ttl      time.Duration
	renewal  time.Duration
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	released atomic.Bool
}

// AcquireLock attempts to acquire a distributed lock on the given key using
// SETNX. If successful, a background goroutine starts renewing the lock at
// 60% of the TTL interval. The caller must call Release on the returned lock
// when done.
//
// Returns the lock and true if the lock was acquired, or the lock and false
// if another caller holds the lock. If the SETNX call itself fails, an error
// is returned.
func AcquireLock(
	ctx context.Context,
	client goredis.UniversalClient,
	key string,
	token string,
	ttl time.Duration,
) (*DistributedLock, bool, error) {
	acquired, err := client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}

	renewInterval := time.Duration(float64(ttl) * 0.6)
	if renewInterval < time.Millisecond {
		renewInterval = time.Millisecond
	}

	renewCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))

	l := &DistributedLock{
		client:  client,
		key:     key,
		token:   token,
		ttl:     ttl,
		renewal: renewInterval,
		cancel:  cancel,
	}

	// Start the renewal goroutine.
	l.wg.Add(1)
	go l.renewLoop(renewCtx)

	return l, true, nil
}

// renewLoop periodically extends the lock's TTL. It stops when the context
// is cancelled (which happens in Release) and must NOT call Release itself.
func (l *DistributedLock) renewLoop(ctx context.Context) {
	defer l.wg.Done()
	ticker := time.NewTicker(l.renewal)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Best-effort renewal. If this fails (e.g., network blip),
			// the lock will expire naturally at its TTL.
			renewCtx, cancel := context.WithTimeout(ctx, l.renewal)
			_ = l.client.Expire(renewCtx, l.key, l.ttl)
			cancel()
		}
	}
}

// Release releases the distributed lock. It is idempotent: calling Release
// multiple times is safe and only the first call has any effect.
//
// Release stops the renewal goroutine and then attempts to delete the lock
// key in Redis only if the token still matches (preventing accidental deletion
// of a lock held by another caller).
func (l *DistributedLock) Release(ctx context.Context) error {
	if !l.released.CompareAndSwap(false, true) {
		return nil // Already released.
	}

	// Stop the renewal goroutine.
	l.cancel()
	l.wg.Wait()

	// Release the lock in Redis, but only if we still hold it.
	releaseCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Use a Lua script to ensure we only delete our own lock.
	// This is the standard Redis pattern for safe lock release.
	script := goredis.NewScript(`
		local current = redis.call("GET", KEYS[1])
		if current == false then return 0 end
		if current ~= ARGV[1] then return 0 end
		redis.call("DEL", KEYS[1])
		return 1
	`)
	_, _ = script.Run(releaseCtx, l.client, []string{l.key}, l.token).Result()
	return nil
}

// GenerateToken creates a cryptographically random token for lock
// identification. The token is used to ensure that only the lock holder
// can release the lock.
func GenerateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
