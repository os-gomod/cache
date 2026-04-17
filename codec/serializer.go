package codec

// Codec defines the interface for serializing and deserializing cache values.
type Codec[T any] interface {
	Encode(v T, buf []byte) ([]byte, error)
	Decode(data []byte) (T, error)
	ContentType() string
}

// RawCodec is a pass-through codec that returns byte slices unchanged.
type RawCodec struct{}

// Encode returns v unchanged as a byte slice.
func (RawCodec) Encode(v, _ []byte) ([]byte, error) { return v, nil }

// Decode returns data unchanged.
func (RawCodec) Decode(data []byte) ([]byte, error) { return data, nil }

// ContentType returns the MIME type for raw binary data.
func (RawCodec) ContentType() string { return "application/octet-stream" }
