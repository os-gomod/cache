package contracts

import (
	"context"
	"testing"
	"time"
)

// mockCache implements Cache for interface compliance testing.
type mockCache struct {
	name string
}

func (m *mockCache) Get(_ context.Context, key string) ([]byte, error) {
	if key == "hit" {
		return []byte("value"), nil
	}
	return nil, nil
}

func (m *mockCache) GetMulti(_ context.Context, keys ...string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, k := range keys {
		if k == "hit" {
			result[k] = []byte("value")
		}
	}
	return result, nil
}

func (m *mockCache) Exists(_ context.Context, key string) (bool, error) {
	return key == "hit", nil
}

func (m *mockCache) TTL(_ context.Context, _ string) (time.Duration, error) {
	return 5 * time.Minute, nil
}

func (m *mockCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

func (m *mockCache) SetMulti(_ context.Context, _ map[string][]byte, _ time.Duration) error {
	return nil
}

func (m *mockCache) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockCache) DeleteMulti(_ context.Context, _ ...string) error {
	return nil
}

func (m *mockCache) CompareAndSwap(
	_ context.Context,
	_ string,
	_, _ []byte,
	_ time.Duration,
) (bool, error) {
	return true, nil
}

func (m *mockCache) SetNX(_ context.Context, _ string, _ []byte, _ time.Duration) (bool, error) {
	return true, nil
}

func (m *mockCache) Increment(_ context.Context, _ string, _ int64) (int64, error) {
	return 1, nil
}

func (m *mockCache) Decrement(_ context.Context, _ string, _ int64) (int64, error) {
	return -1, nil
}

func (m *mockCache) GetSet(_ context.Context, _ string, _ []byte, _ time.Duration) ([]byte, error) {
	return []byte("old"), nil
}

func (m *mockCache) Keys(_ context.Context, _ string) ([]string, error) {
	return []string{"a", "b"}, nil
}

func (m *mockCache) Clear(_ context.Context) error {
	return nil
}

func (m *mockCache) Size(_ context.Context) (int64, error) {
	return 42, nil
}

func (m *mockCache) Ping(_ context.Context) error {
	return nil
}

func (m *mockCache) Close(_ context.Context) error {
	return nil
}

func (m *mockCache) Closed() bool {
	return false
}

func (m *mockCache) Name() string {
	return m.name
}

func (m *mockCache) Stats() StatsSnapshot {
	return StatsSnapshot{Hits: 10, Misses: 5, Items: 42}
}

// mockReadOnly implements ReadOnly for interface compliance testing.
type mockReadOnly struct{}

func (m *mockReadOnly) Get(_ context.Context, _ string) ([]byte, error) { return nil, nil }
func (m *mockReadOnly) GetMulti(_ context.Context, _ ...string) (map[string][]byte, error) {
	return map[string][]byte{}, nil
}
func (m *mockReadOnly) Exists(_ context.Context, _ string) (bool, error)       { return false, nil }
func (m *mockReadOnly) TTL(_ context.Context, _ string) (time.Duration, error) { return 0, nil }
func (m *mockReadOnly) Ping(_ context.Context) error                           { return nil }
func (m *mockReadOnly) Close(_ context.Context) error                          { return nil }
func (m *mockReadOnly) Closed() bool                                           { return false }
func (m *mockReadOnly) Name() string                                           { return "ro" }

func (m *mockReadOnly) Stats() StatsSnapshot { return StatsSnapshot{} }

// mockReadWrite implements ReadWrite for interface compliance testing.
type mockReadWrite struct {
	mockReadOnly
}

func (m *mockReadWrite) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

func (m *mockReadWrite) SetMulti(_ context.Context, _ map[string][]byte, _ time.Duration) error {
	return nil
}
func (m *mockReadWrite) Delete(_ context.Context, _ string) error         { return nil }
func (m *mockReadWrite) DeleteMulti(_ context.Context, _ ...string) error { return nil }

func TestInterfaceCompliance(t *testing.T) {
	tests := []struct {
		name   string
		assert func()
	}{
		{
			name: "Cache interface satisfied by mockCache",
			assert: func() {
				var _ Cache = &mockCache{name: "test"}
			},
		},
		{
			name: "ReadOnly interface satisfied by mockReadOnly",
			assert: func() {
				var _ ReadOnly = &mockReadOnly{}
			},
		},
		{
			name: "ReadWrite interface satisfied by mockReadWrite",
			assert: func() {
				var _ ReadWrite = &mockReadWrite{}
			},
		},
		{
			name: "Reader interface satisfied by mockCache",
			assert: func() {
				var _ Reader = &mockCache{}
			},
		},
		{
			name: "Writer interface satisfied by mockCache",
			assert: func() {
				var _ Writer = &mockCache{}
			},
		},
		{
			name: "AtomicOps interface satisfied by mockCache",
			assert: func() {
				var _ AtomicOps = &mockCache{}
			},
		},
		{
			name: "Scanner interface satisfied by mockCache",
			assert: func() {
				var _ Scanner = &mockCache{}
			},
		},
		{
			name: "Lifecycle interface satisfied by mockCache",
			assert: func() {
				var _ Lifecycle = &mockCache{}
			},
		},
		{
			name: "StatsProvider interface satisfied by mockCache",
			assert: func() {
				var _ StatsProvider = &mockCache{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert()
		})
	}
}

func TestOperationFields(t *testing.T) {
	op := Operation{
		Name:     "get",
		Key:      "user:123",
		KeyCount: 1,
		Backend:  "redis",
	}

	if op.Name != "get" {
		t.Errorf("expected Name=get, got %s", op.Name)
	}
	if op.Key != "user:123" {
		t.Errorf("expected Key=user:123, got %s", op.Key)
	}
	if op.KeyCount != 1 {
		t.Errorf("expected KeyCount=1, got %d", op.KeyCount)
	}
	if op.Backend != "redis" {
		t.Errorf("expected Backend=redis, got %s", op.Backend)
	}
}

func TestResultFields(t *testing.T) {
	result := Result{
		Value:    []byte("hello"),
		Hit:      true,
		ByteSize: 5,
		Latency:  10 * time.Millisecond,
		Err:      nil,
	}

	if !result.Hit {
		t.Error("expected Hit=true")
	}
	if result.ByteSize != 5 {
		t.Errorf("expected ByteSize=5, got %d", result.ByteSize)
	}
	if string(result.Value) != "hello" {
		t.Errorf("expected Value=hello, got %s", result.Value)
	}
}

func TestStatsSnapshotHitRate(t *testing.T) {
	tests := []struct {
		name     string
		snapshot StatsSnapshot
		wantRate float64
	}{
		{
			name:     "no operations",
			snapshot: StatsSnapshot{Hits: 0, Misses: 0},
			wantRate: 0,
		},
		{
			name:     "all hits",
			snapshot: StatsSnapshot{Hits: 100, Misses: 0},
			wantRate: 1.0,
		},
		{
			name:     "all misses",
			snapshot: StatsSnapshot{Hits: 0, Misses: 100},
			wantRate: 0,
		},
		{
			name:     "50/50",
			snapshot: StatsSnapshot{Hits: 50, Misses: 50},
			wantRate: 0.5,
		},
		{
			name:     "75 percent hits",
			snapshot: StatsSnapshot{Hits: 75, Misses: 25},
			wantRate: 0.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.snapshot.HitRate()
			if got != tt.wantRate {
				t.Errorf("HitRate() = %v, want %v", got, tt.wantRate)
			}
		})
	}
}

func TestMockCacheOperations(t *testing.T) {
	ctx := context.Background()
	c := &mockCache{name: "test"}

	// Test Get hit
	val, err := c.Get(ctx, "hit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "value" {
		t.Errorf("expected value, got %s", val)
	}

	// Test Get miss
	val, err = c.Get(ctx, "miss")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil for miss, got %v", val)
	}

	// Test GetMulti
	multi, err := c.GetMulti(ctx, "hit", "miss")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(multi) != 1 {
		t.Errorf("expected 1 result, got %d", len(multi))
	}

	// Test Exists
	exists, err := c.Exists(ctx, "hit")
	if err != nil || !exists {
		t.Error("expected hit to exist")
	}

	exists, err = c.Exists(ctx, "miss")
	if err != nil || exists {
		t.Error("expected miss to not exist")
	}

	// Test Name
	if c.Name() != "test" {
		t.Errorf("expected name=test, got %s", c.Name())
	}

	// Test Stats
	stats := c.Stats()
	if stats.Hits != 10 || stats.Misses != 5 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}
