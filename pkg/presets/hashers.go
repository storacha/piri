package presets

import (
	"crypto/sha256"
	"hash"

	"github.com/multiformats/go-multicodec"
)

var HasherRegistry = map[string]func() hash.Hash{
	multicodec.Sha2_256.String(): sha256.New,
}
