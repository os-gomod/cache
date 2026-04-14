package codec

import (
	"strconv"

	"github.com/os-gomod/cache/internal/unsafeopt"
)

// ---------------------------------------------------------------------------
// StringCodec
// ---------------------------------------------------------------------------

// StringCodec converts between string and []byte with zero-allocation
// decoding using unsafe byte-slice sharing.
type StringCodec struct{}

// Encode converts a string to []byte. The returned slice shares memory with
// the input string (the caller must not modify it).
func (StringCodec) Encode(v string, _ []byte) ([]byte, error) {
	return unsafeopt.StringToBytes(v), nil
}

// Decode converts []byte to string without allocating a new copy.
//
// Safe: caller must not mutate data after Decode — the returned string
// shares the underlying bytes.
func (StringCodec) Decode(data []byte) (string, error) {
	return unsafeopt.BytesToString(data), nil
}

// ContentType returns "text/plain".
func (StringCodec) ContentType() string { return "text/plain" }

// ---------------------------------------------------------------------------
// Int64Codec
// ---------------------------------------------------------------------------

// Int64Codec converts between int64 and its decimal string representation.
type Int64Codec struct{}

// Encode formats an int64 as a decimal byte slice using the provided buf as
// scratch space.
func (Int64Codec) Encode(v int64, buf []byte) ([]byte, error) {
	s := strconv.FormatInt(v, 10)
	if len(buf) < len(s) {
		buf = make([]byte, len(s))
	}
	n := copy(buf, s)
	return buf[:n], nil
}

// Decode parses a decimal byte slice into an int64.
func (Int64Codec) Decode(data []byte) (int64, error) {
	return strconv.ParseInt(unsafeopt.BytesToString(data), 10, 64)
}

// ContentType returns "text/plain".
func (Int64Codec) ContentType() string { return "text/plain" }

// ---------------------------------------------------------------------------
// Float64Codec
// ---------------------------------------------------------------------------

// Float64Codec converts between float64 and its string representation.
type Float64Codec struct{}

// Encode formats a float64 as a byte slice using the provided buf as scratch
// space. It uses 'g' format for the most compact representation.
func (Float64Codec) Encode(v float64, buf []byte) ([]byte, error) {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if len(buf) < len(s) {
		buf = make([]byte, len(s))
	}
	n := copy(buf, s)
	return buf[:n], nil
}

// Decode parses a byte slice into a float64.
func (Float64Codec) Decode(data []byte) (float64, error) {
	return strconv.ParseFloat(unsafeopt.BytesToString(data), 64)
}

// ContentType returns "text/plain".
func (Float64Codec) ContentType() string { return "text/plain" }
