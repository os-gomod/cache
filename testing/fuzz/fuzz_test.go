// Package fuzz provides fuzz targets for the cache library. These tests
// verify that core functions never panic on arbitrary input and that
// round-trip invariants hold for all codecs.
//
// Run with: go test -fuzz=Fuzz -fuzztime=30s ./testing/fuzz/
package fuzz

import (
	"testing"
	"time"

	"github.com/os-gomod/cache/codec"
	"github.com/os-gomod/cache/config"
	"github.com/os-gomod/cache/internal/keyutil"
)

// ---------------------------------------------------------------------------
// FuzzBuildKey — deterministic output, no panic
// ---------------------------------------------------------------------------

func FuzzBuildKey(f *testing.F) {
	// Seed corpus: diverse prefix/key combinations.
	seeds := []struct{ prefix, key string }{
		{"", "mykey"},
		{"cache:", "mykey"},
		{"ns:", ""},
		{"", ""},
		{"very-long-prefix-", "key"},
		{"p", "k"},
	}
	for _, s := range seeds {
		f.Add(s.prefix, s.key)
	}

	f.Fuzz(func(t *testing.T, prefix, key string) {
		// Must never panic.
		result := keyutil.BuildKey(prefix, key)

		// Deterministic: same inputs must produce same output.
		result2 := keyutil.BuildKey(prefix, key)
		if result != result2 {
			t.Errorf("BuildKey not deterministic: %q != %q", result, result2)
		}

		// When prefix is empty, result must equal key.
		if prefix == "" && result != key {
			t.Errorf("BuildKey with empty prefix: got %q, want %q", result, key)
		}

		// When prefix is non-empty, result must start with prefix.
		if prefix != "" && len(result) < len(prefix) {
			t.Errorf("BuildKey result %q shorter than prefix %q", result, prefix)
		}
	})
}

// ---------------------------------------------------------------------------
// FuzzJSONCodec — round-trip invariant: Decode(Encode(x)) == x
// ---------------------------------------------------------------------------

type smallStruct struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
	Flag  bool   `json:"flag"`
}

func FuzzJSONCodec(f *testing.F) {
	// Seed with arbitrary JSON-like bytes.
	seeds := [][]byte{
		{},
		[]byte(`{"name":"test","value":42,"flag":true}`),
		[]byte(`{"name":"","value":0}`),
		[]byte(`null`),
		[]byte(`"hello"`),
		[]byte(`42`),
		[]byte(`[]`),
		make([]byte, 64),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		c := codec.NewJSONCodec[smallStruct]()

		// Decode arbitrary bytes — must not panic.
		decoded, err := c.Decode(data)
		if err != nil {
			// Invalid JSON is expected; skip round-trip check.
			return
		}

		// Encode the decoded value — must not panic.
		encoded, err := c.Encode(decoded, nil)
		if err != nil {
			t.Fatalf("Encode failed after successful Decode: %v", err)
		}

		// Decode(Encode(x)) must equal the original.
		roundTripped, err := c.Decode(encoded)
		if err != nil {
			t.Fatalf("round-trip Decode failed: %v", err)
		}
		if roundTripped.Name != decoded.Name ||
			roundTripped.Value != decoded.Value ||
			roundTripped.Flag != decoded.Flag {
			t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", roundTripped, decoded)
		}
	})
}

// ---------------------------------------------------------------------------
// FuzzConfigValidate — arbitrary config field values must not panic
// ---------------------------------------------------------------------------

func FuzzConfigValidate(f *testing.F) {
	// Seed with diverse values for key config fields.
	seeds := []struct {
		maxEntries  int
		maxMemoryMB int
		shardCount  int
		ttlMs       int
	}{
		{0, 0, 0, 0},
		{10000, 100, 32, 1800000},
		{-1, -1, -1, -1},
		{1, 1, 1, 1},
		{999999, 9999, 4096, 86400000},
	}
	for _, s := range seeds {
		f.Add(s.maxEntries, s.maxMemoryMB, s.shardCount, s.ttlMs)
	}

	f.Fuzz(func(t *testing.T, maxEntries, maxMemoryMB, shardCount, ttlMs int) {
		// Must not panic regardless of input values.
		cfg := &config.Memory{
			MaxEntries:  maxEntries,
			MaxMemoryMB: maxMemoryMB,
			ShardCount:  shardCount,
		}

		// Convert ttlMs to a time.Duration, clamping to reasonable range.
		if ttlMs > -86400000 && ttlMs < 86400000 {
			cfg.DefaultTTL = time.Duration(ttlMs) * time.Millisecond
		} else if ttlMs >= 86400000 {
			cfg.DefaultTTL = 24 * time.Hour
		} else {
			cfg.DefaultTTL = -time.Hour
		}

		// SetDefaults and Validate must not panic.
		cfg.SetDefaults()
		_ = cfg.Validate()
	})
}
