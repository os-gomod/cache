package codec

import "github.com/vmihailenco/msgpack/v5"

// MsgPack is a MessagePack codec for efficient binary serialization.
type MsgPack[T any] struct{}

// NewMsgPack creates a new MessagePack codec.
func NewMsgPack[T any]() *MsgPack[T] {
	return &MsgPack[T]{}
}

// Encode serializes v to MessagePack bytes.
func (c *MsgPack[T]) Encode(v T, _ []byte) ([]byte, error) {
	return msgpack.Marshal(v)
}

// Decode deserializes MessagePack bytes into a value of type T.
func (c *MsgPack[T]) Decode(data []byte) (T, error) {
	var v T
	err := msgpack.Unmarshal(data, &v)
	return v, err
}

// ContentType returns the MIME type for MessagePack-encoded data.
func (c *MsgPack[T]) ContentType() string { return "application/x-msgpack" }
