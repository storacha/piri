package storage

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"

	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/service/storage/ucan"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var log = logging.Logger("storage")

func NewUCANServer(storageService Service, options ...server.Option) (server.ServerView[server.Service], error) {
	options = append(
		options,
		ucan.BlobAllocate(storageService),
		ucan.BlobAccept(storageService),
		ucan.PDPInfo(storageService),
		ucan.ReplicaAllocate(storageService),
	)

	return server.NewServer(storageService.ID(), options...)
}

func NewUCANRetrievalServer(storageService Service, options ...retrieval.Option) (server.ServerView[retrieval.Service], error) {
	allocations := storageService.Blobs().Allocations()
	var blobs blobstore.BlobGetter = storageService.Blobs().Store()
	pdp := storageService.PDP()
	// When PDP is enabled, the blobstore is keyed by piece hash, so adapt it to
	// resolve a blob hash to a piece hash before fetching.
	if pdp != nil {
		blobs = &pieceStoreAdapter{allocations, pdp.PieceFinder(), blobs}
	}

	options = append(
		options,
		retrieval.WithServiceMethod(ucan.SpaceContentRetrieve(allocations, blobs)),
	)

	return retrieval.NewServer(storageService.ID(), options...)
}

type pieceStoreAdapter struct {
	allocations allocationstore.AllocationStore
	pieceFinder piecefinder.PieceFinder
	pieces      blobstore.BlobGetter
}

func (psa *pieceStoreAdapter) Get(ctx context.Context, digest multihash.Multihash, opts ...blobstore.GetOption) (blobstore.Object, error) {
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
