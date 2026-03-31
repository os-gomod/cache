package layered

import (
	"context"
	"time"
)

// startWriteBackWorkers launches goroutines that drain wbCh and persist jobs
// to L2.  Workers stop when wbCh is closed (in Close).
// The worker context is detached from cancellation so background writes can
// outlive a short-lived initialisation context.
func (c *Cache) startWriteBackWorkers(ctx context.Context) {
	workerCtx := context.WithoutCancel(ctx)
	for range c.cfg.WriteBackWorkers {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			for job := range c.wbCh {
				_ = c.l2.Set(workerCtx, job.key, job.value, job.ttl)
				c.stats.WriteBackFlush()
			}
		}()
	}
}

// enqueueWriteBack attempts a non-blocking send to wbCh.
// Drops the job and increments the drop counter when the queue is full.
func (c *Cache) enqueueWriteBack(key string, value []byte, ttl time.Duration) error {
	select {
	case c.wbCh <- wbJob{key: key, value: value, ttl: ttl}:
		c.stats.WriteBackEnqueue()
	default:
		c.stats.WriteBackDrop()
	}
	return nil
}
