package codec

import "github.com/vmihailenco/msgpack/v5"

type MsgPack[T any] struct{}

// NewMsgPack creates a new MessagePack codec for type T.
func NewMsgPack[T any]() *MsgPack[T] {
	return &MsgPack[T]{}
}

// Encode marshals v to MessagePack.
func (c *MsgPack[T]) Encode(v T, _ []byte) ([]byte, error) {
	return msgpack.Marshal(v)
}

// Decode unmarshals MessagePack data into a value of type T.
func (c *MsgPack[T]) Decode(data []byte) (T, error) {
	var v T
	err := msgpack.Unmarshal(data, &v)
	return v, err
}

// ContentType returns "application/x-msgpack".
func (c *MsgPack[T]) ContentType() string { return "application/x-msgpack" }

// // Register it in your codec registry (add this line wherever you register JSON)
// func init() {
// 	Register("msgpack", MsgPack{})
// }
