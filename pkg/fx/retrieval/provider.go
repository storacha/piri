package retrieval

import (
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/retrieval"
	"github.com/storacha/piri/pkg/service/retrieval/ucan"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var Module = fx.Module("retrieval",
	fx.Provide(
		fx.Annotate(
			NewRetrievalService,
			fx.As(new(ucan.SpaceContentRetrievalService)),
		),
	),
)

// RetrievalServiceParams contains all dependencies for the retrieval service
type RetrievalServiceParams struct {
	fx.In

	ID          principal.Signer
	Allocations allocationstore.AllocationStore
	Blobs       blobstore.BlobGetter
	Pieces      blobstore.PDPStore
	PDP         pdp.PDP `optional:"true"`
}

func NewRetrievalService(params RetrievalServiceParams) *retrieval.RetrievalService {
	blobs := params.Blobs
	// When PDP is enabled, blobs are stored in the piece store and keyed by piece
	// hash. We need to adapt it to resolve a blob hash to a piece hash before
	// fetching.
	if params.PDP != nil {
		blobs = retrieval.NewPieceStoreAdapter(
			params.Allocations,
			params.PDP.PieceFinder(),
			params.Pieces,
		)
	}
	return retrieval.New(params.ID, blobs, params.Allocations)
}
