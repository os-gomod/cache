package serialization

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// ---------------------------------------------------------------------------
// Codec interface – generic typed codecs
// ---------------------------------------------------------------------------

func TestJSONCodec_RoundTrip_Primitive(t *testing.T) {
	codec := NewJSONCodec[string]()

	encoded, err := codec.Encode("hello world", nil)
	require.NoError(t, err)
	assert.JSONEq(t, `"hello world"`, string(encoded))

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, "hello world", decoded)
}

func TestJSONCodec_RoundTrip_Struct(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	codec := NewJSONCodec[Person]()
	original := Person{Name: "Alice", Age: 30}

	encoded, err := codec.Encode(original, nil)
	require.NoError(t, err)

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestJSONCodec_RoundTrip_Slice(t *testing.T) {
	codec := NewJSONCodec[[]int]()
	original := []int{1, 2, 3, 4, 5}

	encoded, err := codec.Encode(original, nil)
	require.NoError(t, err)

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestJSONCodec_Decode_Empty(t *testing.T) {
	codec := NewJSONCodec[string]()
	_, err := codec.Decode(nil)
	assert.Error(t, err)

	var cerr *cacheerrors.CacheError
	assert.ErrorAs(t, err, &cerr)
	assert.Equal(t, cacheerrors.CodeDeserialize, cerr.Code)
}

func TestJSONCodec_Decode_InvalidJSON(t *testing.T) {
	codec := NewJSONCodec[string]()
	_, err := codec.Decode([]byte("{invalid"))
	assert.Error(t, err)
}

func TestJSONCodec_Name(t *testing.T) {
	assert.Equal(t, "json", NewJSONCodec[any]().Name())
}

// ---------------------------------------------------------------------------
// MsgpackCodec
// ---------------------------------------------------------------------------

func TestMsgpackCodec_RoundTrip_Primitive(t *testing.T) {
	codec := NewMsgpackCodec[string]()

	encoded, err := codec.Encode("hello msgpack", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, "hello msgpack", decoded)
}

func TestMsgpackCodec_RoundTrip_Struct(t *testing.T) {
	type Point struct {
		X float64 `msgpack:"x"`
		Y float64 `msgpack:"y"`
	}

	codec := NewMsgpackCodec[Point]()
	original := Point{X: 1.5, Y: 2.5}

	encoded, err := codec.Encode(original, nil)
	require.NoError(t, err)

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestMsgpackCodec_RoundTrip_Map(t *testing.T) {
	codec := NewMsgpackCodec[map[string]any]()
	original := map[string]any{"key": "value", "num": 42}

	encoded, err := codec.Encode(original, nil)
	require.NoError(t, err)

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestMsgpackCodec_Decode_Empty(t *testing.T) {
	codec := NewMsgpackCodec[string]()
	_, err := codec.Decode(nil)
	assert.Error(t, err)

	var cerr *cacheerrors.CacheError
	assert.ErrorAs(t, err, &cerr)
	assert.Equal(t, cacheerrors.CodeDeserialize, cerr.Code)
}

func TestMsgpackCodec_Name(t *testing.T) {
	assert.Equal(t, "msgpack", NewMsgpackCodec[any]().Name())
}

// ---------------------------------------------------------------------------
// Zero-allocation codecs
// ---------------------------------------------------------------------------

func TestRawCodec_RoundTrip(t *testing.T) {
	codec := RawCodec{}
	original := []byte("raw bytes \x00\x01\x02")

	encoded, err := codec.Encode(original, nil)
	require.NoError(t, err)
	assert.Equal(t, original, encoded)

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestRawCodec_Empty(t *testing.T) {
	codec := RawCodec{}

	encoded, err := codec.Encode([]byte{}, nil)
	require.NoError(t, err)
	assert.Empty(t, encoded)

	decoded, err := codec.Decode(nil)
	require.NoError(t, err)
	assert.Empty(t, decoded)
}

func TestRawCodec_Name(t *testing.T) {
	assert.Equal(t, "raw", RawCodec{}.Name())
}

func TestStringCodec_RoundTrip(t *testing.T) {
	codec := StringCodec{}
	original := "hello, world! 🌍"

	encoded, err := codec.Encode(original, nil)
	require.NoError(t, err)

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestStringCodec_Name(t *testing.T) {
	assert.Equal(t, "string", StringCodec{}.Name())
}

func TestInt64Codec_RoundTrip(t *testing.T) {
	codec := Int64Codec{}
	values := []int64{0, 1, -1, 42, 9223372036854775807, -9223372036854775808}

	for _, v := range values {
		encoded, err := codec.Encode(v, nil)
		require.NoError(t, err, "encode %d", v)

		decoded, err := codec.Decode(encoded)
		require.NoError(t, err, "decode %d", v)
		assert.Equal(t, v, decoded)
	}
}

func TestInt64Codec_ScratchReuse(t *testing.T) {
	codec := Int64Codec{}
	scratch := make([]byte, 0, 32)

	enc1, err := codec.Encode(100, scratch)
	require.NoError(t, err)
	assert.Equal(t, "100", string(enc1))

	// Reuse the same scratch buffer (which may have been extended).
	enc2, err := codec.Encode(-999, enc1[:0])
	require.NoError(t, err)
	assert.Equal(t, "-999", string(enc2))
}

func TestInt64Codec_Decode_Invalid(t *testing.T) {
	codec := Int64Codec{}
	_, err := codec.Decode([]byte("not-a-number"))
	assert.Error(t, err)
}

func TestInt64Codec_Name(t *testing.T) {
	assert.Equal(t, "int64", Int64Codec{}.Name())
}

func TestFloat64Codec_RoundTrip(t *testing.T) {
	codec := Float64Codec{}
	values := []float64{0.0, 1.5, -3.14, 1e10, 1e-10}

	for _, v := range values {
		encoded, err := codec.Encode(v, nil)
		require.NoError(t, err, "encode %f", v)

		decoded, err := codec.Decode(encoded)
		require.NoError(t, err, "decode %f", v)
		assert.Equal(t, v, decoded)
	}
}

func TestFloat64Codec_Decode_Invalid(t *testing.T) {
	codec := Float64Codec{}
	_, err := codec.Decode([]byte("nan-value"))
	assert.Error(t, err)
}

func TestFloat64Codec_Name(t *testing.T) {
	assert.Equal(t, "float64", Float64Codec{}.Name())
}

func TestBoolCodec_RoundTrip(t *testing.T) {
	codec := BoolCodec{}

	for _, v := range []bool{true, false} {
		encoded, err := codec.Encode(v, nil)
		require.NoError(t, err, "encode %v", v)

		decoded, err := codec.Decode(encoded)
		require.NoError(t, err, "decode %v", v)
		assert.Equal(t, v, decoded)
	}
}

func TestBoolCodec_Decode_Aliases(t *testing.T) {
	codec := BoolCodec{}
	testCases := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"t", true},
		{"f", false},
		{"T", true},
		{"F", false},
	}

	for _, tc := range testCases {
		got, err := codec.Decode([]byte(tc.input))
		require.NoError(t, err, "input=%q", tc.input)
		assert.Equal(t, tc.want, got, "input=%q", tc.input)
	}
}

func TestBoolCodec_Decode_Invalid(t *testing.T) {
	codec := BoolCodec{}
	_, err := codec.Decode([]byte("yes"))
	assert.Error(t, err)
}

func TestBoolCodec_Name(t *testing.T) {
	assert.Equal(t, "bool", BoolCodec{}.Name())
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	jsonCodec := NewAnyJSONCodec()
	err := reg.Register("json", jsonCodec)
	require.NoError(t, err)

	got, ok := reg.Get("json")
	require.True(t, ok)
	assert.Equal(t, "json", got.Name())
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	reg := NewRegistry()

	err := reg.Register("json", NewAnyJSONCodec())
	require.NoError(t, err)

	err = reg.Register("json", NewAnyMsgpackCodec())
	assert.Error(t, err)
}

func TestRegistry_Get_NotFound(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_Default_AutoFirst(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register("msgpack", NewAnyMsgpackCodec()))

	def, ok := reg.Default()
	require.True(t, ok)
	assert.Equal(t, "msgpack", def.Name())
}

func TestRegistry_SetDefault(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register("json", NewAnyJSONCodec()))
	require.NoError(t, reg.Register("msgpack", NewAnyMsgpackCodec()))

	err := reg.SetDefault("msgpack")
	require.NoError(t, err)

	def, ok := reg.Default()
	require.True(t, ok)
	assert.Equal(t, "msgpack", def.Name())
}

func TestRegistry_SetDefault_NotFound(t *testing.T) {
	reg := NewRegistry()
	err := reg.SetDefault("nonexistent")
	assert.Error(t, err)
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register("msgpack", NewAnyMsgpackCodec()))
	require.NoError(t, reg.Register("json", NewAnyJSONCodec()))

	list := reg.List()
	assert.Equal(t, []string{"json", "msgpack"}, list)
}

func TestRegistry_Len(t *testing.T) {
	reg := NewRegistry()
	assert.Equal(t, 0, reg.Len())

	require.NoError(t, reg.Register("json", NewAnyJSONCodec()))
	assert.Equal(t, 1, reg.Len())
}

func TestRegistry_MustGet(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register("json", NewAnyJSONCodec()))
	assert.Equal(t, "json", reg.MustGet("json").Name())

	assert.Panics(t, func() { reg.MustGet("missing") })
}

// ---------------------------------------------------------------------------
// VersionedCodec
// ---------------------------------------------------------------------------

func TestVersionedCodec_RoundTrip(t *testing.T) {
	base := NewJSONCodec[string]()
	vc := NewVersionedCodec(base, 1)

	encoded, err := vc.Encode("hello", nil)
	require.NoError(t, err)

	// Verify version header.
	assert.Equal(t, uint16(1), binary.BigEndian.Uint16(encoded[:2]))

	decoded, err := vc.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, "hello", decoded)
}

func TestVersionedCodec_Migration(t *testing.T) {
	base := NewJSONCodec[string]()

	// Current version is 3.
	vc := NewVersionedCodec(base, 3)

	// Register a migration from version 1: data was just raw bytes,
	// we wrap it in JSON quotes.
	vc.WithMigration(1, func(data []byte) ([]byte, error) {
		// v1 stored raw strings, v3 stores JSON strings.
		wrapped := make([]byte, 0, len(data)+2)
		wrapped = append(wrapped, '"')
		wrapped = append(wrapped, data...)
		wrapped = append(wrapped, '"')
		return wrapped, nil
	})

	// Simulate v1-encoded data: version header 1 + raw "hello".
	v1Data := make([]byte, 2+len("hello"))
	binary.BigEndian.PutUint16(v1Data[:2], 1)
	copy(v1Data[2:], "hello")

	decoded, err := vc.Decode(v1Data)
	require.NoError(t, err)
	assert.Equal(t, "hello", decoded)
}

func TestVersionedCodec_NoMigration(t *testing.T) {
	base := NewJSONCodec[string]()
	vc := NewVersionedCodec(base, 5)

	// Simulate v2-encoded data with no migration registered.
	v2Data := make([]byte, 4)
	binary.BigEndian.PutUint16(v2Data[:2], 2)
	copy(v2Data[2:], "xx")

	_, err := vc.Decode(v2Data)
	assert.Error(t, err)
}

func TestVersionedCodec_TooShort(t *testing.T) {
	base := NewJSONCodec[string]()
	vc := NewVersionedCodec(base, 1)

	_, err := vc.Decode([]byte{0x00})
	assert.Error(t, err)
}

func TestVersionedCodec_Name(t *testing.T) {
	vc := NewVersionedCodec(NewJSONCodec[string](), 7)
	assert.Equal(t, "json:v7", vc.Name())
	assert.Equal(t, uint16(7), vc.Version())
}

// ---------------------------------------------------------------------------
// BufPool
// ---------------------------------------------------------------------------

func TestBufPool_GetAndPut(t *testing.T) {
	pool := NewBufPool(256)

	buf := pool.Get()
	require.NotNil(t, buf)
	assert.Equal(t, 0, len(*buf))
	assert.GreaterOrEqual(t, cap(*buf), 256)

	// Write into the buffer.
	*buf = append(*buf, []byte("hello")...)

	// Return to pool.
	pool.Put(buf)

	// Get again – should have been reset.
	buf2 := pool.Get()
	assert.Equal(t, 0, len(*buf2), "buffer should be reset after Put")
}

func TestBufPool_NilPut(t *testing.T) {
	pool := NewBufPool(128)
	pool.Put(nil) // should not panic
}

func TestBufPool_DefaultSize(t *testing.T) {
	pool := NewBufPool(0)
	assert.Equal(t, 4096, pool.Size())

	buf := pool.Get()
	assert.GreaterOrEqual(t, cap(*buf), 4096)
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register("json", NewAnyJSONCodec()))

	done := make(chan struct{})

	// Concurrent reads.
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				reg.Get("json")
				reg.Default()
				reg.List()
				reg.Len()
			}
		}()
	}

	// Concurrent writes (will fail on duplicate but should not crash).
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			reg.Register(fmt.Sprintf("json-extra-%d", i), NewAnyJSONCodec())
		}()
	}

	// Wait for all goroutines.
	for i := 0; i < 15; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for goroutines")
		}
	}
}

func TestBufPool_ConcurrentAccess(t *testing.T) {
	pool := NewBufPool(64)
	done := make(chan struct{})

	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				buf := pool.Get()
				*buf = append(*buf, "data"...)
				pool.Put(buf)
			}
		}()
	}

	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for goroutines")
		}
	}
}
