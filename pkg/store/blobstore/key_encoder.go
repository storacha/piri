package blobstore

import (
	"path/filepath"

	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/piri/pkg/store/objectstore/flatfs"
)

// KeyEncoder defines how to encode blob keys for a specific backend.
type KeyEncoder interface {
	EncodeKey(digest multihash.Multihash) string
}

// Base32KeyEncoder encodes keys as base32 (S3/MinIO compatible with IPFS boxo).
// This is the default encoder for flatfs backends.
type Base32KeyEncoder struct{}

func (Base32KeyEncoder) EncodeKey(digest multihash.Multihash) string {
	// Adapted from
	// https://github.com/ipfs/boxo/blob/8c17f11f399062878a8093f12cedce56877dbb6f/datastore/dshelp/key.go#L13-L18
	b32, _ := multibase.Encode(multibase.Base32, digest)
	return b32[1:] // strip base indicator
}

// PlainKeyEncoder encodes keys as plain digest format using digestutil.Format.
// This is the default encoder for in-memory and datastore backends.
type PlainKeyEncoder struct{}

func (PlainKeyEncoder) EncodeKey(digest multihash.Multihash) string {
	return digestutil.Format(digest)
}

// Base32FlatFSKeyEncoder is a [Base32KeyEncoder] that also adds a sharding
// directory prefix and ".data" suffix, making it compatible with FlatFS
// NextToLast(2) sharding.
type Base32FlatFSKeyEncoder struct {
	shard flatfs.ShardFunc
}

func NewBase32FlatFSKeyEncoder() *Base32FlatFSKeyEncoder {
	return &Base32FlatFSKeyEncoder{shard: flatfs.NextToLast(2).Func()}
}

func (f *Base32FlatFSKeyEncoder) EncodeKey(digest multihash.Multihash) string {
	b32 := Base32KeyEncoder{}.EncodeKey(digest)
	dir := f.shard(b32)
	return filepath.Join(dir, b32+".data")
}
