package cache_test

import (
	"context"
	"testing"
	"time"

	cache "github.com/os-gomod/cache"
	"github.com/os-gomod/cache/codec"
	"github.com/os-gomod/cache/memory"
)

func TestMemory_CreatesBackend(t *testing.T) {
	c, err := cache.Memory(
		memory.WithMaxEntries(100),
		memory.WithTTL(10*time.Minute),
		memory.WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	if c == nil {
		t.Fatal("Memory() returned nil")
	}
	if c.Name() != "memory" {
		t.Errorf("Name() = %q, want %q", c.Name(), "memory")
	}
}

func TestMemory_WithOptions(t *testing.T) {
	c, err := cache.Memory(
		memory.WithMaxEntries(50),
		memory.WithTTL(time.Hour),
		memory.WithCleanupInterval(0),
		memory.WithShards(4),
	)
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	// Verify cache works
	ctx := context.Background()
	if err := c.Set(ctx, "key", []byte("value"), 0); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	val, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(val) != "value" {
		t.Errorf("Get = %q, want %q", val, "value")
	}
}

func TestMemory_CRUD(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	// Miss
	_, err = c.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for miss")
	}

	// Set + Get
	if err := c.Set(ctx, "key", []byte("hello"), 0); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	val, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(val) != "hello" {
		t.Errorf("Get = %q, want %q", val, "hello")
	}

	// Delete
	if err := c.Delete(ctx, "key"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	_, err = c.Get(ctx, "key")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestGet_Generic(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	// Set raw
	jc := codec.NewJSONCodec[Person]()
	data, err := jc.Encode(Person{Name: "Alice", Age: 30}, nil)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if err := c.Set(ctx, "person:1", data, 0); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Get with generic
	p, err := cache.Get(ctx, c, "person:1", jc)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.Name != "Alice" || p.Age != 30 {
		t.Errorf("Get = %+v, want {Alice 30}", p)
	}
}

func TestSet_Generic(t *testing.T) {
	c, err := cache.Memory(memory.WithCleanupInterval(0))
	if err != nil {
		t.Fatalf("Memory() error: %v", err)
	}
	defer c.Close(context.Background())
	ctx := context.Background()

	type Person struct {
		Name string `json:"name"`
	}

	jc := codec.NewJSONCodec[Person]()
	err = cache.Set(ctx, c, "person:2", Person{Name: "Bob"}, 0, jc)
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	p, err := cache.Get(ctx, c, "person:2", jc)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.Name != "Bob" {
		t.Errorf("Get = %+v, want {Bob}", p)
	}
}
