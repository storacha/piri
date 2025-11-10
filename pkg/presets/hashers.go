package presets

import (
	"crypto/sha256"
	"crypto/sha512"
	"hash"

	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/multiformats/go-multicodec"
	"golang.org/x/crypto/sha3"
)

var HasherRegistry = map[string]hash.Hash{
	multicodec.Sha2_256.String():                     sha256.New(),
	multicodec.Sha2_512.String():                     sha512.New(),
	multicodec.Sha3_256.String():                     sha3.New256(),
	multicodec.Sha3_512.String():                     sha3.New512(),
	multicodec.Fr32Sha256Trunc254Padbintree.String(): &commp.Calc{},
	multicodec.Sha2_256Trunc254Padded.String():       &commp.Calc{},
}
