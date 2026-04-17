package codec

import (
	"strconv"

	"github.com/os-gomod/cache/internal/unsafeopt"
)

// StringCodec is a zero-allocation codec for string values using unsafe string/byte conversion.
type StringCodec struct{}

// Encode converts a string to bytes without allocation.
func (StringCodec) Encode(v string, _ []byte) ([]byte, error) {
	return unsafeopt.StringToBytes(v), nil
}

// Decode converts bytes to a string without allocation.
func (StringCodec) Decode(data []byte) (string, error) {
	return unsafeopt.BytesToString(data), nil
}

// ContentType returns the MIME type for plain text data.
func (StringCodec) ContentType() string { return "text/plain" }

// Int64Codec is a zero-allocation codec for int64 values.
type Int64Codec struct{}

// Encode converts an int64 to its decimal string representation.
func (Int64Codec) Encode(v int64, buf []byte) ([]byte, error) {
	s := strconv.FormatInt(v, 10)
	if len(buf) < len(s) {
		buf = make([]byte, len(s))
	}
	n := copy(buf, s)
	return buf[:n], nil
}

// Decode parses a decimal string into an int64.
func (Int64Codec) Decode(data []byte) (int64, error) {
	return strconv.ParseInt(unsafeopt.BytesToString(data), 10, 64)
}

// ContentType returns the MIME type for plain text data.
func (Int64Codec) ContentType() string { return "text/plain" }

// Float64Codec is a zero-allocation codec for float64 values.
type Float64Codec struct{}

// Encode converts a float64 to its string representation.
func (Float64Codec) Encode(v float64, buf []byte) ([]byte, error) {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if len(buf) < len(s) {
		buf = make([]byte, len(s))
	}
	n := copy(buf, s)
	return buf[:n], nil
}

// Decode parses a string into a float64.
func (Float64Codec) Decode(data []byte) (float64, error) {
	return strconv.ParseFloat(unsafeopt.BytesToString(data), 64)
}

// ContentType returns the MIME type for plain text data.
func (Float64Codec) ContentType() string { return "text/plain" }
