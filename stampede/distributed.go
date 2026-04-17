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

// DistributedLock represents a Redis-based distributed lock with automatic renewal.
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

// AcquireLock attempts to acquire a distributed lock in Redis.
// Returns the lock, a boolean indicating acquisition, and any error.
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
	l.wg.Add(1)
	go l.renewLoop(renewCtx)
	return l, true, nil
}

func (l *DistributedLock) renewLoop(ctx context.Context) {
	defer l.wg.Done()
	ticker := time.NewTicker(l.renewal)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			renewCtx, cancel := context.WithTimeout(ctx, l.renewal)
			_ = l.client.Expire(renewCtx, l.key, l.ttl)
			cancel()
		}
	}
}

// Release releases the distributed lock. It is safe to call multiple times.
// Only the holder that created the lock can release it.
func (l *DistributedLock) Release(ctx context.Context) error {
	if !l.released.CompareAndSwap(false, true) {
		return nil
	}
	l.cancel()
	l.wg.Wait()
	releaseCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
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

// GenerateToken generates a cryptographically random token for use as a lock value.
func GenerateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
