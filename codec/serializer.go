// Package codec provides pluggable serialization for typed cache access.
// All codecs implement the Codec[T] interface with a scratch-buffer-aware
// Encode method that enables zero-allocation-on-hot-path encoding for
// primitive types.
package codec

// Codec serializes and deserializes values of type T. Implementations must
// be safe for concurrent use.
//
// The Encode method accepts a scratch buffer (buf) that implementations
// should reuse when possible to reduce allocations. The returned bytes may
// or may not share storage with buf — callers must not assume either.
type Codec[T any] interface {
	// Encode serializes v into buf. Implementations should use the provided
	// buf as scratch space and return the resulting bytes (may or may not be
	// buf). If buf is nil or too small, the implementation allocates.
	Encode(v T, buf []byte) ([]byte, error)

	// Decode deserializes data into a value of type T.
	Decode(data []byte) (T, error)

	// ContentType returns the MIME type for this codec (e.g.,
	// "application/json").
	ContentType() string
}

// RawCodec is a Codec[[]byte] that is a no-op — values pass through
// without any serialization, resulting in zero allocations.
type RawCodec struct{}

func (RawCodec) Encode(v, _ []byte) ([]byte, error) { return v, nil }
func (RawCodec) Decode(data []byte) ([]byte, error) { return data, nil }
func (RawCodec) ContentType() string                { return "application/octet-stream" }
