package serializer

import (
	"encoding/json"

	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-ucanto/core/ipld/codec/cbor"
)

type Serializer[T any] interface {
	Serialize(val T) ([]byte, error)
	Deserialize(data []byte) (T, error)
}

type IPLDCBOR[T any] struct {
	Typ  schema.Type
	Opts []bindnode.Option
}

func (i *IPLDCBOR[T]) Serialize(val T) ([]byte, error) {
	return cbor.Encode(&val, i.Typ, i.Opts...)
}

func (i *IPLDCBOR[T]) Deserialize(data []byte) (T, error) {
	var out T
	if err := cbor.Decode(data, &out, i.Typ, i.Opts...); err != nil {
		return out, err
	}
	return out, nil
}

type JSON[T any] struct{}

func (J JSON[T]) Serialize(val T) ([]byte, error) {
	return json.Marshal(val)
}

func (J JSON[T]) Deserialize(data []byte) (T, error) {
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}
