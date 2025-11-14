package piece_test

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"io"
	"path/filepath"
	"testing"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/piece"
	"github.com/storacha/piri/pkg/pdp/service/models"
)

type resolverFixture struct {
	ctx      context.Context
	db       *gorm.DB
	resolver types.PieceResolverAPI
}

func TestMultihashToCommpV2CID(t *testing.T) {
	var pieceCID, commpCID cid.Cid
	var pieceMH multihash.Multihash
	{
		size := 10 * 1024
		c := &commp.Calc{}
		data := testutil.RandomBytes(t, size)

		n, err := io.Copy(c, bytes.NewReader(data))
		require.NoError(t, err)
		require.EqualValues(t, size, n)

		digest, _, err := c.Digest()
		require.NoError(t, err)

		pieceCID, err = commcid.DataCommitmentToPieceCidv2(digest, uint64(size))
		require.NoError(t, err)
		pieceMH = pieceCID.Hash()
	}

	{

		commpCID = piece.MultihashToCommpCID(pieceMH)

	}

	require.Equal(t, pieceCID.String(), commpCID.String())
}

func newResolverFixture(t *testing.T) resolverFixture {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "resolver.db")

	db, err := gormdb.New(dbPath)
	require.NoError(t, err)

	err = models.AutoMigrateDB(t.Context(), db)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	resolver, err := piece.NewStoreResolver(piece.StoreResolverParams{
		DB: db,
	})
	require.NoError(t, err)
	return resolverFixture{
		ctx:      context.Background(),
		db:       db,
		resolver: resolver,
	}
}

func randomBytes(t *testing.T, size int) []byte {
	bytes := make([]byte, size)
	n, err := crand.Read(bytes)
	require.NoError(t, err)
	require.Equal(t, size, n)
	return bytes
}

func randomMultihash(t *testing.T, data []byte) multihash.Multihash {
	out, err := multihash.Sum(data, multihash.SHA2_256, -1)
	require.NoError(t, err)
	return out
}

func TestStoreResolverResolveWithCommpMapping(t *testing.T) {
	fx := newResolverFixture(t)

	commpCID := mustTestCommpCID(t)
	require.Equal(t, uint64(multicodec.Fr32Sha256Trunc254Padbintree), commpCID.Prefix().MhType)

	size := 10 * 1024
	resolvedHash := randomMultihash(t, randomBytes(t, size))

	entry := models.PDPPieceMHToCommp{
		Mhash: resolvedHash,
		Size:  int64(size),
		Commp: commpCID.String(),
	}
	require.NoError(t, fx.db.Create(&entry).Error)

	got, ok, err := fx.resolver.ResolveToBlob(fx.ctx, commpCID.Hash())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, resolvedHash, got)
}

func TestStoreResolverResolveNotFound(t *testing.T) {
	fx := newResolverFixture(t)

	commpCID := mustTestCommpCID(t)

	got, ok, err := fx.resolver.Resolve(fx.ctx, commpCID.Hash())
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, got)
}

func mustTestCommpCID(t *testing.T) cid.Cid {
	t.Helper()

	c := &commp.Calc{}
	size := 10 * 1024
	data := testutil.RandomBytes(t, size)

	n, err := io.Copy(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.EqualValues(t, size, n)

	digest, paddedSize, err := c.Digest()
	require.NoError(t, err)
	// not required, but for completeness
	treeHeight, _, err := commcid.PayloadSizeToV1TreeHeightAndPadding(uint64(size))
	require.NoError(t, err)
	expectedPaddedSize := uint64(32) << treeHeight
	require.Equal(t, paddedSize, expectedPaddedSize)

	pieceCID, err := commcid.DataCommitmentToPieceCidv2(digest, uint64(size))
	require.NoError(t, err)

	return pieceCID
}
