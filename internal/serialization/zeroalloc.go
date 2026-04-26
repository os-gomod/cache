package serialization

import (
	"strconv"

	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// RawCodec is a zero-allocation pass-through codec for []byte values.
// The Encode method returns the input slice directly (no copy), making
// it ideal when the cache stores opaque blobs that don't need
// serialization.
//
// WARNING: Encode does NOT copy the input. If the caller mutates the
// byte slice after encoding, the cached data may be corrupted. Use
// bytes.Clone on the input if immutability is required.
type RawCodec struct{}

// Encode returns value as-is with no serialization overhead. The scratch
// buffer is ignored.
func (RawCodec) Encode(value, _scratch []byte) ([]byte, error) {
	return value, nil
}

// Decode returns data as-is with no deserialization overhead.
func (RawCodec) Decode(data []byte) ([]byte, error) {
	return data, nil
}

// Name returns the codec identifier "raw".
func (RawCodec) Name() string {
	return "raw"
}

// ---------------------------------------------------------------------------

// StringCodec is a zero-copy codec for string values. It converts
// between string and []byte with a single allocation on Encode and
// zero allocations on Decode.
type StringCodec struct{}

// Encode converts value to []byte. The scratch buffer is ignored.
func (StringCodec) Encode(value string, _scratch []byte) ([]byte, error) {
	return []byte(value), nil
}

// Decode converts data to a string with zero allocations.
func (StringCodec) Decode(data []byte) (string, error) {
	return string(data), nil
}

// Name returns the codec identifier "string".
func (StringCodec) Name() string {
	return "string"
}

// ---------------------------------------------------------------------------

// Int64Codec is a zero-copy (after initial allocation) codec for int64
// values. It serializes integers as base-10 ASCII strings, which are
// human-readable and compatible with Redis INCR/DECR semantics.
type Int64Codec struct{}

// Encode appends the base-10 representation of value to scratch (or
// allocates a new buffer if scratch is nil/insufficient).
func (Int64Codec) Encode(value int64, scratch []byte) ([]byte, error) {
	return strconv.AppendInt(scratch, value, 10), nil
}

// Decode parses a base-10 integer from data.
func (Int64Codec) Decode(data []byte) (int64, error) {
	v, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0, cacheerrors.Factory.New(cacheerrors.CodeDeserialize, "Int64Codec.Decode", "",
			"invalid int64 value: "+string(data), err)
	}
	return v, nil
}

// Name returns the codec identifier "int64".
func (Int64Codec) Name() string {
	return "int64"
}

// ---------------------------------------------------------------------------

// Float64Codec serializes float64 values as base-10 ASCII strings
// using the shortest representation that round-trips exactly.
type Float64Codec struct{}

// Encode appends the decimal representation of value to scratch.
func (Float64Codec) Encode(value float64, scratch []byte) ([]byte, error) {
	return strconv.AppendFloat(scratch, value, 'f', -1, 64), nil
}

// Decode parses a float64 from data.
func (Float64Codec) Decode(data []byte) (float64, error) {
	v, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return 0, cacheerrors.Factory.New(cacheerrors.CodeDeserialize, "Float64Codec.Decode", "",
			"invalid float64 value: "+string(data), err)
	}
	return v, nil
}

// Name returns the codec identifier "float64".
func (Float64Codec) Name() string {
	return "float64"
}

// ---------------------------------------------------------------------------

// BoolCodec serializes bool values as the ASCII strings "true" or "false".
type BoolCodec struct{}

// Encode returns the byte representation of the boolean value.
func (BoolCodec) Encode(value bool, _scratch []byte) ([]byte, error) {
	if value {
		return []byte("true"), nil
	}
	return []byte("false"), nil
}

// Decode parses a boolean from data. Accepted values are "true", "false",
// "1", "0", "t", "f", "T", "F" (case-insensitive for true/false/1/0).
func (BoolCodec) Decode(data []byte) (bool, error) {
	switch string(data) {
	case "true", "1", "t", "T":
		return true, nil
	case "false", "0", "f", "F":
		return false, nil
	default:
		return false, cacheerrors.Factory.New(cacheerrors.CodeDeserialize, "BoolCodec.Decode", "",
			"invalid boolean value: "+string(data), nil)
	}
}

// Name returns the codec identifier "bool".
func (BoolCodec) Name() string {
	return "bool"
}
