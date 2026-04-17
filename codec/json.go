// Package codec provides serialization and deserialization codecs for typed cache values.
package codec

import (
	"bytes"
	"encoding/json"
	"sync"
)

// JSONCodec is a JSON-based codec that uses sync.Pool for buffer reuse.
type JSONCodec[T any] struct {
	pool sync.Pool
}

// NewJSONCodec creates a new pooled JSON codec.
func NewJSONCodec[T any]() *JSONCodec[T] {
	return &JSONCodec[T]{
		pool: sync.Pool{
			New: func() any { return new(bytes.Buffer) },
		},
	}
}

// Encode serializes v to JSON bytes.
func (c *JSONCodec[T]) Encode(v T, _ []byte) ([]byte, error) {
	buf := c.pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer c.pool.Put(buf)
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return nil, err
	}
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

// Decode deserializes JSON bytes into a value of type T.
func (c *JSONCodec[T]) Decode(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

// ContentType returns the MIME type for JSON-encoded data.
func (c *JSONCodec[T]) ContentType() string { return "application/json" }
