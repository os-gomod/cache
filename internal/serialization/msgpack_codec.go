//nolint:dupl // structural similarity is intentional
package serialization

import (
	"math"

	"github.com/vmihailenco/msgpack/v5"

	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// MsgpackCodec provides a MessagePack-based serialization codec using
// github.com/vmihailenco/msgpack/v5. MessagePack is a binary format
// that is typically 30-50% smaller and 3-5× faster than JSON, making
// it an excellent choice for high-throughput cache workloads.
//
// For primitive types ([]byte, string, int64, float64, bool) consider
// using the zero-allocation codecs instead for maximum performance.
type MsgpackCodec[T any] struct{}

// NewMsgpackCodec creates a new MsgpackCodec[T] for serializing values of type T.
func NewMsgpackCodec[T any]() *MsgpackCodec[T] {
	return &MsgpackCodec[T]{}
}

// Encode serializes value to MessagePack bytes. The scratch buffer is
// not reused because msgpack.Marshal allocates internally.
func (*MsgpackCodec[T]) Encode(value T, _scratch []byte) ([]byte, error) {
	data, err := msgpack.Marshal(value)
	if err != nil {
		return nil, cacheerrors.Factory.SerializeError("MsgpackCodec.Encode", err)
	}
	return data, nil
}

// Decode deserialises MessagePack bytes into a value of type T.
func (*MsgpackCodec[T]) Decode(data []byte) (T, error) {
	var zero T
	if len(data) == 0 {
		return zero, cacheerrors.Factory.New(cacheerrors.CodeDeserialize, "MsgpackCodec.Decode", "",
			"empty input", nil)
	}

	var value T
	if err := msgpack.Unmarshal(data, &value); err != nil {
		return zero, cacheerrors.Factory.DeserializeError("MsgpackCodec.Decode", err)
	}
	value = normalizeDecodedInterfaces(value)
	return value, nil
}

// Name returns the codec identifier "msgpack".
func (*MsgpackCodec[T]) Name() string {
	return "msgpack"
}

// ---------------------------------------------------------------------------
// anyCodec – a non-generic adapter for use with Registry (Codec[any])
// ---------------------------------------------------------------------------

// anyMsgpackCodec wraps MsgpackCodec for Codec[any] registration.
type anyMsgpackCodec struct {
	codec *MsgpackCodec[any]
}

func (a *anyMsgpackCodec) Encode(value any, scratch []byte) ([]byte, error) {
	return a.codec.Encode(value, scratch)
}

func (a *anyMsgpackCodec) Decode(data []byte) (any, error) {
	return a.codec.Decode(data)
}

func (a *anyMsgpackCodec) Name() string {
	return a.codec.Name()
}

// NewAnyMsgpackCodec returns a Codec[any] that serializes arbitrary values
// as MessagePack.
func NewAnyMsgpackCodec() Codec[any] {
	return &anyMsgpackCodec{codec: NewMsgpackCodec[any]()}
}

func normalizeDecodedInterfaces[T any](value T) T {
	normalized := normalizeDecodedValue(any(value))
	if cast, ok := normalized.(T); ok {
		return cast
	}
	return value
}

func normalizeDecodedValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeDecodedValue(item)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = normalizeDecodedValue(item)
		}
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		if v >= math.MinInt && v <= math.MaxInt {
			return int(v)
		}
		return v
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		if uint64(v) <= math.MaxInt {
			return int(v)
		}
		return v
	case uint64:
		if v <= math.MaxInt {
			return int(v)
		}
		return v
	default:
		return value
	}
}
