package ucan

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/service/retrieval/handlers/spacecontent"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var log = logging.Logger("retrieval/ucan")

type SpaceContentRetrievalService interface {
	Allocations() allocationstore.AllocationStore
	Blobs() blobstore.BlobGetter
}

func SpaceContentRetrieve(retrievalService SpaceContentRetrievalService) retrieval.Option {
	return retrieval.WithServiceMethod(
		content.RetrieveAbility,
		retrieval.Provide(
			content.Retrieve,
			func(ctx context.Context, cap ucan.Capability[content.RetrieveCaveats], inv invocation.Invocation, iCtx server.InvocationContext, request retrieval.Request) (result.Result[content.RetrieveOk, failure.IPLDBuilderFailure], fx.Effects, retrieval.Response, error) {
				space, err := did.Parse(cap.With())
				if err != nil {
					return nil, nil, retrieval.Response{}, fmt.Errorf("parsing space DID: %w", err)
				}

				nb := cap.Nb()
				digest := nb.Blob.Digest
				digestStr := digestutil.Format(digest)
				start := nb.Range.Start
				end := nb.Range.End

				log := log.With(
					"iss", inv.Issuer().DID().String(),
					"can", content.RetrieveAbility,
					"with", space.String(),
					"digest", digestStr,
					"range", fmt.Sprintf("%d-%d", start, end),
				)

				_, err = retrievalService.Allocations().Get(ctx, digest, space)
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						log.Debugw("allocation not found", "status", http.StatusNotFound)
						notFoundErr := content.NewNotFoundError(fmt.Sprintf("allocation not found: %s", digestStr))
						res := result.Error[content.RetrieveOk, failure.IPLDBuilderFailure](notFoundErr)
						resp := retrieval.NewResponse(http.StatusNotFound, nil, nil)
						return res, nil, resp, nil
					}
					log.Errorw("getting allocation", "error", err)
					return nil, nil, retrieval.Response{}, fmt.Errorf("getting allocation: %w", err)
				}

				res, resp, err := spacecontent.Retrieve(ctx, retrievalService.Blobs(), inv, digest, &blobstore.Range{Start: start, End: &end})
				if err != nil {
					return nil, nil, retrieval.Response{}, err
				}
				return res, nil, resp, nil
			},
		),
	)
}
