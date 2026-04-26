package layered

import (
	"context"
	"time"
)

// writeBackJob represents a deferred L2 write operation.
type writeBackJob struct {
	key   string
	value []byte
	ttl   time.Duration
}

// writeBackWorker processes jobs from the write-back queue and writes them
// to L2. If a write fails, it is retried once after a short backoff.
func (s *Store) writeBackWorker() {
	defer s.wg.Done()

	const maxRetries = 2
	const retryBackoff = 10 * time.Millisecond

	for job := range s.wbCh {
		var writeErr error
		for attempt := range maxRetries {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			writeErr = s.l2.Set(ctx, job.key, job.value, job.ttl)
			cancel()

			if writeErr == nil {
				s.stats.WriteBackFlush()
				break
			}

			// Retry after backoff
			if attempt < maxRetries-1 {
				select {
				case <-time.After(retryBackoff):
					continue
				default:
					break
				}
			}
		}

		if writeErr != nil {
			s.stats.L2Error()
			s.stats.ErrorOp()
			_ = writeErr // Error logged via stats
		}
	}
}
