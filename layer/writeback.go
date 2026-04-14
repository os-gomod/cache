package layer

import (
	"context"
)

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
