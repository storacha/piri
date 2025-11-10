package service

import (
	"bytes"
	"io"
	"testing"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
	"github.com/samber/lo"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"
)

func TestDoCommp(t *testing.T) {
	size := uint64(10 * 1024)
	t.Run("Sha2_256Trunc254Padded", func(t *testing.T) {
		data := testutil.RandomBytes(t, int(size))
		c := &commp.Calc{}

		n, err := io.Copy(c, bytes.NewReader(data))
		require.NoError(t, err)
		require.EqualValues(t, size, n)

		commpDigest, paddedSize, err := c.Digest()
		require.NoError(t, err)

		v2cid, err := commcid.DataCommitmentToPieceCidv2(commpDigest, size)
		require.NoError(t, err)

		commpMh, err := mh.Encode(commpDigest, uint64(multicodec.Sha2_256Trunc254Padded))
		require.NoError(t, err)

		actualV2CID, actualPaddedSize, err := doCommp(commpMh, bytes.NewReader(data), size)
		require.NoError(t, err)

		require.Equal(t, paddedSize, actualPaddedSize)
		require.Equal(t, v2cid.String(), actualV2CID.String())

	})
	t.Run("Fr32Sha256Trunc254Padbintree", func(t *testing.T) {
		data := testutil.RandomBytes(t, int(size))
		c := &commp.Calc{}

		n, err := io.Copy(c, bytes.NewReader(data))
		require.NoError(t, err)
		require.EqualValues(t, size, n)

		commpDigest, paddedSize, err := c.Digest()
		require.NoError(t, err)

		v2cid, err := commcid.DataCommitmentToPieceCidv2(commpDigest, size)
		require.NoError(t, err)

		commpMh, err := mh.Encode(commpDigest, uint64(multicodec.Fr32Sha256Trunc254Padbintree))
		require.NoError(t, err)

		actualV2CID, actualPaddedSize, err := doCommp(commpMh, bytes.NewReader(data), size)
		require.NoError(t, err)

		require.Equal(t, paddedSize, actualPaddedSize)
		require.Equal(t, v2cid.String(), actualV2CID.String())
	})
	t.Run("user-specified cid", func(t *testing.T) {
		data := testutil.RandomBytes(t, int(size))

		c := &commp.Calc{}
		n, err := io.Copy(c, bytes.NewReader(data))
		require.NoError(t, err)
		require.EqualValues(t, size, n)

		commpDigest, paddedSize, err := c.Digest()
		require.NoError(t, err)

		v2cid, err := commcid.DataCommitmentToPieceCidv2(commpDigest, size)
		require.NoError(t, err)

		userCID := randomCID(t, data)
		actualV2CID, actualPaddedSize, err := doCommp(userCID.Hash(), bytes.NewReader(data), size)
		require.NoError(t, err)

		require.Equal(t, paddedSize, actualPaddedSize)
		require.Equal(t, v2cid.String(), actualV2CID.String())
	})
}

func randomCID(t *testing.T, data []byte) cid.Cid {
	return cid.NewCidV1(cid.Raw, randomMultihash(t, data))
}

func randomMultihash(t *testing.T, data []byte) mh.Multihash {
	return lo.Must(mh.Sum(data, mh.SHA2_256, -1))
}
