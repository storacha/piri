package testutil

import (
	crand "crypto/rand"
	"fmt"
	"net/url"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/principal/ed25519/signer"
	"github.com/stretchr/testify/require"
)

func MultihashOfBytes(t testing.TB, bytes []byte) mh.Multihash {
	c, err := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   mh.SHA2_256,
		MhLength: -1,
	}.Sum(bytes)
	require.NoError(t, err)
	return cidlink.Link{Cid: c}.Hash()
}

func RandomBytes(t *testing.T, size int) []byte {
	bytes := make([]byte, size)
	_, err := crand.Read(bytes)
	require.NoError(t, err)
	return bytes
}

func RandomCID(t *testing.T) datamodel.Link {
	bytes := RandomBytes(t, 10)
	c, err := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   mh.SHA2_256,
		MhLength: -1,
	}.Sum(bytes)
	require.NoError(t, err)
	return cidlink.Link{Cid: c}
}

func RandomMultihash(t *testing.T) mh.Multihash {
	return RandomCID(t).(cidlink.Link).Hash()
}

func RandomSigner(t *testing.T) principal.Signer {
	s, err := signer.Generate()
	require.NoError(t, err)
	return s
}

func RandomDID(t *testing.T) did.DID {
	return RandomSigner(t).DID()
}

var assignedPorts = map[int]struct{}{}

// RandomLocalURL finds a free port and will not generate another URL with the
// same port until test cleanup, even if no service is bound to it.
func RandomLocalURL(t *testing.T) url.URL {
	var port int
	for {
		port = GetFreePort(t)
		if _, ok := assignedPorts[port]; !ok {
			assignedPorts[port] = struct{}{}
			t.Cleanup(func() { delete(assignedPorts, port) })
			break
		}
	}
	pubURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	require.NoError(t, err)
	return *pubURL
}
