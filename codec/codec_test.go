package codec

import (
	"testing"
)

// ---------------------------------------------------------------------------
// RawCodec tests
// ---------------------------------------------------------------------------

func TestRawCodec_Encode(t *testing.T) {
	c := RawCodec{}
	data := []byte("hello")
	out, err := c.Encode(data, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if string(out) != "hello" {
		t.Errorf("Encode = %q, want %q", out, "hello")
	}
}

func TestRawCodec_Decode(t *testing.T) {
	c := RawCodec{}
	data := []byte("hello")
	out, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if string(out) != "hello" {
		t.Errorf("Decode = %q, want %q", out, "hello")
	}
}

func TestRawCodec_ContentType(t *testing.T) {
	c := RawCodec{}
	if ct := c.ContentType(); ct != "application/octet-stream" {
		t.Errorf("ContentType = %q, want %q", ct, "application/octet-stream")
	}
}

// ---------------------------------------------------------------------------
// JSONCodec tests
// ---------------------------------------------------------------------------

type testUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestJSONCodec_EncodeDecode(t *testing.T) {
	c := NewJSONCodec[testUser]()
	original := testUser{ID: 1, Name: "Alice", Email: "alice@example.com"}

	data, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ID != original.ID || decoded.Name != original.Name ||
		decoded.Email != original.Email {
		t.Errorf("Decode = %+v, want %+v", decoded, original)
	}
}

func TestJSONCodec_ContentType(t *testing.T) {
	c := NewJSONCodec[testUser]()
	if ct := c.ContentType(); ct != "application/json" {
		t.Errorf("ContentType = %q, want %q", ct, "application/json")
	}
}

func TestJSONCodec_PoolReuse(t *testing.T) {
	c := NewJSONCodec[int]()
	for i := 0; i < 100; i++ {
		_, err := c.Encode(i, nil)
		if err != nil {
			t.Fatalf("Encode(%d) failed: %v", i, err)
		}
	}
}

// ---------------------------------------------------------------------------
// StringCodec tests
// ---------------------------------------------------------------------------

func TestStringCodec_EncodeDecode(t *testing.T) {
	c := StringCodec{}
	original := "Hello, World!"

	data, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded != original {
		t.Errorf("Decode = %q, want %q", decoded, original)
	}
}

func TestStringCodec_ContentType(t *testing.T) {
	c := StringCodec{}
	if ct := c.ContentType(); ct != "text/plain" {
		t.Errorf("ContentType = %q, want %q", ct, "text/plain")
	}
}

// ---------------------------------------------------------------------------
// Int64Codec tests
// ---------------------------------------------------------------------------

func TestInt64Codec_EncodeDecode(t *testing.T) {
	c := Int64Codec{}
	original := int64(42)

	data, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded != original {
		t.Errorf("Decode = %d, want %d", decoded, original)
	}
}

func TestInt64Codec_Negative(t *testing.T) {
	c := Int64Codec{}
	original := int64(-999999)

	data, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded != original {
		t.Errorf("Decode = %d, want %d", decoded, original)
	}
}

func TestInt64Codec_ScratchBuffer(t *testing.T) {
	c := Int64Codec{}
	buf := make([]byte, 0, 64)
	data, err := c.Encode(int64(123), buf)
	if err != nil {
		t.Fatalf("Encode with scratch failed: %v", err)
	}
	if string(data) != "123" {
		t.Errorf("Encode = %q, want %q", data, "123")
	}
}

// ---------------------------------------------------------------------------
// Float64Codec tests
// ---------------------------------------------------------------------------

func TestFloat64Codec_EncodeDecode(t *testing.T) {
	c := Float64Codec{}
	original := float64(3.14159)

	data, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded != original {
		t.Errorf("Decode = %g, want %g", decoded, original)
	}
}

func TestFloat64Codec_ScratchBuffer(t *testing.T) {
	c := Float64Codec{}
	buf := make([]byte, 0, 64)
	data, err := c.Encode(float64(2.5), buf)
	if err != nil {
		t.Fatalf("Encode with scratch failed: %v", err)
	}
	if string(data) != "2.5" {
		t.Errorf("Encode = %q, want %q", data, "2.5")
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkRawCodec_Encode(b *testing.B) {
	c := RawCodec{}
	data := []byte("hello world")
	for b.Loop() {
		_, _ = c.Encode(data, nil)
	}
}

func BenchmarkRawCodec_Decode(b *testing.B) {
	c := RawCodec{}
	data := []byte("hello world")
	for b.Loop() {
		_, _ = c.Decode(data)
	}
}

func BenchmarkJSONCodec_Encode(b *testing.B) {
	c := NewJSONCodec[testUser]()
	v := testUser{ID: 1, Name: "Alice", Email: "alice@example.com"}
	for b.Loop() {
		_, _ = c.Encode(v, nil)
	}
}

func BenchmarkJSONCodec_Decode(b *testing.B) {
	c := NewJSONCodec[testUser]()
	data, _ := c.Encode(testUser{ID: 1, Name: "Alice", Email: "alice@example.com"}, nil)
	for b.Loop() {
		_, _ = c.Decode(data)
	}
}

func BenchmarkStringCodec_Encode(b *testing.B) {
	c := StringCodec{}
	s := "Hello, World!"
	for b.Loop() {
		_, _ = c.Encode(s, nil)
	}
}

func BenchmarkStringCodec_Decode(b *testing.B) {
	c := StringCodec{}
	data := []byte("Hello, World!")
	for b.Loop() {
		_, _ = c.Decode(data)
	}
}

func BenchmarkInt64Codec_Encode(b *testing.B) {
	c := Int64Codec{}
	for b.Loop() {
		_, _ = c.Encode(int64(42), nil)
	}
}

func BenchmarkInt64Codec_Decode(b *testing.B) {
	c := Int64Codec{}
	data := []byte("42")
	for b.Loop() {
		_, _ = c.Decode(data)
	}
}

func BenchmarkFloat64Codec_Encode(b *testing.B) {
	c := Float64Codec{}
	for b.Loop() {
		_, _ = c.Encode(3.14159, nil)
	}
}

func BenchmarkFloat64Codec_Decode(b *testing.B) {
	c := Float64Codec{}
	data := []byte("3.14159")
	for b.Loop() {
		_, _ = c.Decode(data)
	}
}

// ---------------------------------------------------------------------------
// Scratch-buffer benchmarks (showing zero-alloc encode path with provided buf)
// ---------------------------------------------------------------------------

func BenchmarkInt64Codec_Encode_Scratch(b *testing.B) {
	c := Int64Codec{}
	buf := make([]byte, 0, 64)
	for b.Loop() {
		_, _ = c.Encode(int64(42), buf)
	}
}

func BenchmarkInt64Codec_Decode_Scratch(b *testing.B) {
	c := Int64Codec{}
	data := []byte("42")
	for b.Loop() {
		_, _ = c.Decode(data)
	}
}

func BenchmarkFloat64Codec_Encode_Scratch(b *testing.B) {
	c := Float64Codec{}
	buf := make([]byte, 0, 64)
	for b.Loop() {
		_, _ = c.Encode(3.14159, buf)
	}
}

func BenchmarkFloat64Codec_Decode_Scratch(b *testing.B) {
	c := Float64Codec{}
	data := []byte("3.14159")
	for b.Loop() {
		_, _ = c.Decode(data)
	}
}

// ---------------------------------------------------------------------------
// Additional edge-case tests
// ---------------------------------------------------------------------------

func TestRawCodec_NilInput(t *testing.T) {
	c := RawCodec{}
	out, err := c.Encode(nil, nil)
	if err != nil {
		t.Fatalf("Encode(nil) failed: %v", err)
	}
	if out != nil {
		t.Errorf("Encode(nil) = %v, want nil", out)
	}

	dec, err := c.Decode(nil)
	if err != nil {
		t.Fatalf("Decode(nil) failed: %v", err)
	}
	if dec != nil {
		t.Errorf("Decode(nil) = %v, want nil", dec)
	}
}

func TestInt64Codec_Zero(t *testing.T) {
	c := Int64Codec{}
	data, err := c.Encode(int64(0), nil)
	if err != nil {
		t.Fatalf("Encode(0) failed: %v", err)
	}
	if string(data) != "0" {
		t.Errorf("Encode(0) = %q, want %q", data, "0")
	}
	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded != 0 {
		t.Errorf("Decode = %d, want 0", decoded)
	}
}

func TestInt64Codec_MaxInt64(t *testing.T) {
	c := Int64Codec{}
	const maxVal = int64(9223372036854775807)
	data, err := c.Encode(maxVal, nil)
	if err != nil {
		t.Fatalf("Encode(max) failed: %v", err)
	}
	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded != maxVal {
		t.Errorf("Decode = %d, want %d", decoded, maxVal)
	}
}

func TestFloat64Codec_Zero(t *testing.T) {
	c := Float64Codec{}
	data, err := c.Encode(float64(0), nil)
	if err != nil {
		t.Fatalf("Encode(0) failed: %v", err)
	}
	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded != 0 {
		t.Errorf("Decode = %g, want 0", decoded)
	}
}

func TestFloat64Codec_Negative(t *testing.T) {
	c := Float64Codec{}
	original := float64(-123.456)
	data, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded != original {
		t.Errorf("Decode = %g, want %g", decoded, original)
	}
}

func TestJSONCodec_PrimitiveInt(t *testing.T) {
	c := NewJSONCodec[int]()
	data, err := c.Encode(42, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded != 42 {
		t.Errorf("Decode = %d, want 42", decoded)
	}
}

func TestJSONCodec_Slice(t *testing.T) {
	c := NewJSONCodec[[]string]()
	original := []string{"a", "b", "c"}
	data, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(decoded) != 3 || decoded[0] != "a" || decoded[1] != "b" || decoded[2] != "c" {
		t.Errorf("Decode = %v, want %v", decoded, original)
	}
}

func TestStringCodec_Empty(t *testing.T) {
	c := StringCodec{}
	data, err := c.Encode("", nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("Encode = %q, want empty", data)
	}
	decoded, err := c.Decode([]byte{})
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded != "" {
		t.Errorf("Decode = %q, want empty", decoded)
	}
}

func TestInt64Codec_InvalidDecode(t *testing.T) {
	c := Int64Codec{}
	_, err := c.Decode([]byte("not-a-number"))
	if err == nil {
		t.Error("expected error decoding invalid int64, got nil")
	}
}

func TestFloat64Codec_InvalidDecode(t *testing.T) {
	c := Float64Codec{}
	_, err := c.Decode([]byte("not-a-float"))
	if err == nil {
		t.Error("expected error decoding invalid float64, got nil")
	}
}

func TestInt64Codec_SmallScratchBuffer(t *testing.T) {
	c := Int64Codec{}
	// buf too small — codec should allocate
	buf := make([]byte, 0, 1)
	data, err := c.Encode(int64(42), buf)
	if err != nil {
		t.Fatalf("Encode with small scratch failed: %v", err)
	}
	if string(data) != "42" {
		t.Errorf("Encode = %q, want %q", data, "42")
	}
}

func TestFloat64Codec_SmallScratchBuffer(t *testing.T) {
	c := Float64Codec{}
	buf := make([]byte, 0, 1)
	data, err := c.Encode(3.14, buf)
	if err != nil {
		t.Fatalf("Encode with small scratch failed: %v", err)
	}
	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded != 3.14 {
		t.Errorf("roundtrip = %g, want 3.14", decoded)
	}
}
