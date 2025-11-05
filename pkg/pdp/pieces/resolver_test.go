package pieces_test

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/pieces"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type resolverFixture struct {
	ctx      context.Context
	db       *gorm.DB
	store    *blobstore.MapBlobstore
	resolver *pieces.StoreResolver
}

func TestRoundTripCommP(t *testing.T) {
	c := &commp.Calc{}
	size := 10 * 1024
	data := testutil.RandomBytes(t, size)

	n, err := io.Copy(c, bytes.NewReader(data))
	require.NoError(t, err)
	require.EqualValues(t, size, n)

	digest, _, err := c.Digest()
	require.NoError(t, err)

	pieceCID, err := commcid.DataCommitmentToPieceCidv2(digest, uint64(size))
	require.NoError(t, err)

	pieceMH := pieceCID.Hash()
	dmh, err := multihash.Decode(pieceMH)
	require.NoError(t, err)

	// TODO: this is probably wrong
	tmp := cid.NewCidV1(dmh.Code, dmh.Digest)
	commpdigest, _, err := commcid.PieceCidV2ToDataCommitment(tmp)
	require.NoError(t, err)

	_ = commpdigest

	_, commpCID, err := cid.CidFromBytes(digest)
	require.NoError(t, err)

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

	store := blobstore.NewMapBlobstore()

	return resolverFixture{
		ctx:      context.Background(),
		db:       db,
		store:    store,
		resolver: pieces.NewStoreResolver(db, store),
	}
}

func TestStoreResolverResolveWithCommpMapping(t *testing.T) {
	fx := newResolverFixture(t)

	commpCID := mustTestCommpCID(t)
	require.Equal(t, uint64(multicodec.Fr32Sha256Trunc254Padbintree), commpCID.Prefix().MhType)

	ihatethis := testutil.RandomCID(t)
	resolvedCID, err := cid.Parse(ihatethis.String())
	require.NoError(t, err)
	/*
		rawMH, err := multihash.Sum([]byte("resolved-payload"), multihash.SHA2_256, -1)
		require.NoError(t, err)
		decoded, err := multihash.Decode(rawMH)
		require.NoError(t, err)
		resolvedCID := cid.NewCidV1(uint64(decoded.Code), decoded.Digest)

	*/

	entry := models.PDPPieceMHToCommp{
		Mhash: resolvedCID.Hash(),
		Size:  int64(len("resolved-payload")),
		Commp: commpCID.String(),
	}
	require.NoError(t, fx.db.Create(&entry).Error)

	got, ok, err := fx.resolver.Resolve(fx.ctx, commpCID.Hash())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, resolvedCID, got)
}

func TestStoreResolverResolveReadsBlobstore(t *testing.T) {
	fx := newResolverFixture(t)

	data := []byte("plain-piece")
	mh, err := multihash.Sum(data, multihash.SHA2_256, -1)
	require.NoError(t, err)
	pieceCID := cid.NewCidV1(uint64(multicodec.Raw), mh)

	require.NoError(t, fx.store.Put(fx.ctx, mh, uint64(len(data)), bytes.NewReader(data)))

	got, ok, err := fx.resolver.Resolve(fx.ctx, pieceCID.Hash())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, pieceCID, got)
}

func TestStoreResolverResolveNotFound(t *testing.T) {
	fx := newResolverFixture(t)

	commpCID := mustTestCommpCID(t)

	got, ok, err := fx.resolver.Resolve(fx.ctx, commpCID.Hash())
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, cid.Undef, got)
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
