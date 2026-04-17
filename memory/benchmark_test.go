package memory

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkGet(b *testing.B) {
	c, _ := New(WithMaxEntries(100000))
	ctx := context.Background()
	c.Set(ctx, "key", []byte("value"), 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(ctx, "key")
	}
}

func BenchmarkSet(b *testing.B) {
	c, _ := New(WithMaxEntries(100000))
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(ctx, fmt.Sprintf("key-%d", i), []byte("value"), 0)
	}
}

func BenchmarkGetParallel(b *testing.B) {
	c, _ := New(WithMaxEntries(100000), WithShards(64))
	ctx := context.Background()
	c.Set(ctx, "key", []byte("value"), 0)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(ctx, fmt.Sprintf("key-%d", i%1000))
			i++
		}
	})
}

func BenchmarkSetParallel(b *testing.B) {
	c, _ := New(WithMaxEntries(100000), WithShards(64))
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Set(ctx, fmt.Sprintf("key-%d", i%100000), []byte("benchmark-value"), 0)
			i++
		}
	})
}

func BenchmarkMixed(b *testing.B) {
	c, _ := New(WithMaxEntries(10000), WithShards(64))
	ctx := context.Background()
	for i := 0; i < 5000; i++ {
		c.Set(ctx, fmt.Sprintf("key-%d", i), []byte("value"), 0)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%3 == 0 {
				c.Set(ctx, fmt.Sprintf("key-%d", i%10000), []byte("value"), 0)
			} else {
				c.Get(ctx, fmt.Sprintf("key-%d", i%10000))
			}
			i++
		}
	})
}

func BenchmarkShardedVsSingle(b *testing.B) {
	for _, shards := range []int{1, 4, 16, 64, 256} {
		b.Run(fmt.Sprintf("shards=%d", shards), func(b *testing.B) {
			c, _ := New(WithMaxEntries(100000), WithShards(shards))
			ctx := context.Background()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					c.Set(ctx, fmt.Sprintf("key-%d", i%10000), []byte("value"), 0)
					i++
				}
			})
		})
	}
}
