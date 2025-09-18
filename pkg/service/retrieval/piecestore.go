package retrieval

import (
	"context"
	"fmt"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

// PieceStoreAdapter adapts a blobstore keyed by piece multihash to a blobstore
// keyed by user hash.
type PieceStoreAdapter struct {
	allocations allocationstore.AllocationStore
	pieceFinder piecefinder.PieceFinder
	pieces      blobstore.BlobGetter
}

func (psa *PieceStoreAdapter) Get(ctx context.Context, digest multihash.Multihash, opts ...blobstore.GetOption) (blobstore.Object, error) {
	allocs, err := psa.allocations.List(ctx, digest, allocationstore.WithLimit(1))
	if err != nil {
		return nil, fmt.Errorf("listing allocations: %w", err)
	}
	if len(allocs) == 0 {
		return nil, store.ErrNotFound
	}
	pieceLink, err := psa.pieceFinder.FindPiece(ctx, digest, allocs[0].Blob.Size)
	if err != nil {
		return nil, fmt.Errorf("finding piece link for %s: %w", digestutil.Format(digest), err)
	}
	pieceDigest := pieceLink.Link().(cidlink.Link).Cid.Hash()
	return psa.pieces.Get(ctx, pieceDigest, opts...)
}

// NewPieceStoreAdapter creates a new adapter that adapts a blobstore keyed by
// piece multihash to a blobstore keyed by user hash (typically sha2-256).
func NewPieceStoreAdapter(allocations allocationstore.AllocationStore, finder piecefinder.PieceFinder, pieces blobstore.BlobGetter) *PieceStoreAdapter {
	return &PieceStoreAdapter{allocations, finder, pieces}
}
