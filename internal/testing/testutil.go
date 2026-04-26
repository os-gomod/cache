package testing

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

// RandomKey generates a random alphanumeric key with the given prefix
// and length. If prefix is empty, "key" is used. The total length
// includes the prefix plus a hyphen separator plus random hex characters.
func RandomKey(prefix string, length int) string {
	if prefix == "" {
		prefix = "key"
	}
	if length <= len(prefix)+1 {
		length = len(prefix) + 9
	}

	b := make([]byte, (length-len(prefix)-1+1)/2)
	_, _ = rand.Read(b)
	randomPart := hex.EncodeToString(b)

	if len(prefix)+1+len(randomPart) > length {
		randomPart = randomPart[:length-len(prefix)-1]
	}

	return prefix + "-" + randomPart
}

// RandomValue generates a random byte slice of the given size.
// The bytes are filled with random data from crypto/rand.
func RandomValue(size int) []byte {
	if size <= 0 {
		size = 32
	}
	b := make([]byte, size)
	_, _ = rand.Read(b)
	return b
}

// Wait sleeps for the given duration. It logs the wait time via t.Log
// so that slow tests are easy to diagnose.
func Wait(t *testing.T, d time.Duration) {
	t.Helper()
	t.Logf("waiting %v for state transition", d)
	time.Sleep(d)
}

// WaitUntil polls a condition function at the given interval until it
// returns true or the timeout expires. This is useful for testing
// eventual-consistency behaviors.
func WaitUntil(t *testing.T, condition func() bool, interval, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("WaitUntil timed out after %v", timeout)
}

// FormatBytes returns a human-readable string representation of a byte
// count (e.g. "1.5 KiB", "2.3 MiB").
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GiB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MiB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KiB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// ParallelGoroutines runs n goroutines in parallel, each calling fn
// with its goroutine index. It blocks until all goroutines complete
// and reports the first non-nil error returned by any goroutine.
func ParallelGoroutines(t *testing.T, n int, fn func(idx int) error) {
	t.Helper()
	t.Run(fmt.Sprintf("parallel_%d_goroutines", n), func(t *testing.T) {
		errCh := make(chan error, n)
		for i := range n {
			go func(idx int) {
				errCh <- fn(idx)
			}(i)
		}
		for i := range n {
			if err := <-errCh; err != nil {
				t.Errorf("goroutine %d failed: %v", i, err)
			}
		}
	})
}
