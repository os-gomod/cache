package codec

import (
	"bytes"
	"encoding/json"
	"sync"
)

// JSONCodec serializes values using encoding/json. It uses a sync.Pool of
// bytes.Buffer to reduce allocations on the encode path.
type JSONCodec[T any] struct {
	pool sync.Pool
}

// NewJSONCodec creates a new JSON codec for type T.
func NewJSONCodec[T any]() *JSONCodec[T] {
	return &JSONCodec[T]{
		pool: sync.Pool{
			New: func() any { return new(bytes.Buffer) },
		},
	}
}

// Encode marshals v to JSON using a pooled buffer. The result is copied out
// of the pooled buffer before it is returned to the pool, so the caller owns
// the returned slice.
//
// For struct types with only value fields, this typically results in ≤ 2
// allocations per call: one for the json.Encoder state and one for the
// output copy. The pooled buffer eliminates the buffer allocation itself.
func (c *JSONCodec[T]) Encode(v T, _ []byte) ([]byte, error) {
	buf := c.pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer c.pool.Put(buf)

	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return nil, err
	}
	// Copy out of the pooled buffer — the caller owns the result.
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

// Decode unmarshals JSON data into a value of type T.
func (c *JSONCodec[T]) Decode(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

// ContentType returns "application/json".
func (c *JSONCodec[T]) ContentType() string { return "application/json" }
