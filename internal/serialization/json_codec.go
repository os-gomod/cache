//nolint:dupl // structural similarity is intentional
package serialization

import (
	"encoding/json"

	cacheerrors "github.com/os-gomod/cache/v2/internal/errors"
)

// JSONCodec provides a JSON-based serialization codec. It uses
// encoding/json which is included in the Go standard library and
// therefore requires no external dependencies.
//
// The scratch buffer is not reused by this implementation because
// json.Marshal allocates internally. Callers seeking zero-alloc
// behaviour should prefer MsgpackCodec or one of the primitive
// codecs in the zeroalloc package.
type JSONCodec[T any] struct{}

// NewJSONCodec creates a new JSONCodec[T] for serializing values of type T.
func NewJSONCodec[T any]() *JSONCodec[T] {
	return &JSONCodec[T]{}
}

// Encode serializes value to JSON bytes. The scratch buffer is ignored
// because encoding/json allocates internally.
func (*JSONCodec[T]) Encode(value T, _scratch []byte) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, cacheerrors.Factory.SerializeError("JSONCodec.Encode", err)
	}
	return data, nil
}

// Decode deserialises JSON bytes into a value of type T.
func (*JSONCodec[T]) Decode(data []byte) (T, error) {
	var zero T
	if len(data) == 0 {
		return zero, cacheerrors.Factory.New(cacheerrors.CodeDeserialize, "JSONCodec.Decode", "",
			"empty input", nil)
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return zero, cacheerrors.Factory.DeserializeError("JSONCodec.Decode", err)
	}
	return value, nil
}

// Name returns the codec identifier "json".
func (*JSONCodec[T]) Name() string {
	return "json"
}

// ---------------------------------------------------------------------------
// anyCodec – a non-generic adapter for use with Registry (Codec[any])
// ---------------------------------------------------------------------------

// anyJSONCodec wraps JSONCodec for Codec[any] registration.
type anyJSONCodec struct {
	codec *JSONCodec[any]
}

func (a *anyJSONCodec) Encode(value any, scratch []byte) ([]byte, error) {
	return a.codec.Encode(value, scratch)
}

func (a *anyJSONCodec) Decode(data []byte) (any, error) {
	return a.codec.Decode(data)
}

func (a *anyJSONCodec) Name() string {
	return a.codec.Name()
}

// NewAnyJSONCodec returns a Codec[any] that serializes arbitrary values as JSON.
func NewAnyJSONCodec() Codec[any] {
	return &anyJSONCodec{codec: NewJSONCodec[any]()}
}
