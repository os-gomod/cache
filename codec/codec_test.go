package codec

import (
	"math"
	"testing"
)

type testStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestRawCodec_EncodeDecode(t *testing.T) {
	c := RawCodec{}
	original := []byte("hello world")
	encoded, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if string(encoded) != string(original) {
		t.Errorf("Encode got %q, want %q", encoded, original)
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if string(decoded) != string(original) {
		t.Errorf("Decode got %q, want %q", decoded, original)
	}
}

func TestRawCodec_ContentType(t *testing.T) {
	if (RawCodec{}).ContentType() != "application/octet-stream" {
		t.Error("unexpected content type")
	}
}

func TestJSONCodec_EncodeDecode(t *testing.T) {
	c := NewJSONCodec[testStruct]()
	original := testStruct{Name: "Alice", Age: 30}
	encoded, err := c.Encode(original, nil)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("Encode returned empty")
	}
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if decoded.Name != original.Name || decoded.Age != original.Age {
		t.Errorf("Decode got %+v, want %+v", decoded, original)
	}
}

func TestJSONCodec_EmptySlice(t *testing.T) {
	c := NewJSONCodec[testStruct]()
	_, err := c.Encode(testStruct{}, nil)
	if err != nil {
		t.Fatalf("Encode empty struct error: %v", err)
	}
}

func TestJSONCodec_ContentType(t *testing.T) {
	c := NewJSONCodec[testStruct]()
	if c.ContentType() != "application/json" {
		t.Error("unexpected content type")
	}
}

func TestStringCodec_EncodeDecode(t *testing.T) {
	c := StringCodec{}
	tests := []string{"hello", "", "unicode: 你好"}
	for _, s := range tests {
		encoded, err := c.Encode(s, nil)
		if err != nil {
			t.Errorf("Encode(%q) error: %v", s, err)
			continue
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Errorf("Decode error: %v", err)
			continue
		}
		if decoded != s {
			t.Errorf("roundtrip %q: got %q", s, decoded)
		}
	}
}

func TestStringCodec_ContentType(t *testing.T) {
	if (StringCodec{}).ContentType() != "text/plain" {
		t.Error("unexpected content type")
	}
}

func TestInt64Codec_EncodeDecode(t *testing.T) {
	c := Int64Codec{}
	tests := []int64{0, -1, 42, math.MaxInt64}
	for _, v := range tests {
		encoded, err := c.Encode(v, nil)
		if err != nil {
			t.Errorf("Encode(%d) error: %v", v, err)
			continue
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Errorf("Decode error: %v", err)
			continue
		}
		if decoded != v {
			t.Errorf("roundtrip %d: got %d", v, decoded)
		}
	}
}

func TestInt64Codec_Decode_Invalid(t *testing.T) {
	c := Int64Codec{}
	_, err := c.Decode([]byte("abc"))
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestInt64Codec_ContentType(t *testing.T) {
	if (Int64Codec{}).ContentType() != "text/plain" {
		t.Error("unexpected content type")
	}
}

func TestFloat64Codec_EncodeDecode(t *testing.T) {
	c := Float64Codec{}
	tests := []float64{0.0, -1.5, 3.14, math.MaxFloat64}
	for _, v := range tests {
		encoded, err := c.Encode(v, nil)
		if err != nil {
			t.Errorf("Encode(%f) error: %v", v, err)
			continue
		}
		decoded, err := c.Decode(encoded)
		if err != nil {
			t.Errorf("Decode error: %v", err)
			continue
		}
		if decoded != v {
			t.Errorf("roundtrip %f: got %f", v, decoded)
		}
	}
}

func TestFloat64Codec_Decode_Invalid(t *testing.T) {
	c := Float64Codec{}
	_, err := c.Decode([]byte("not-a-float"))
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestFloat64Codec_ContentType(t *testing.T) {
	if (Float64Codec{}).ContentType() != "text/plain" {
		t.Error("unexpected content type")
	}
}
