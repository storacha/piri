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
					"client", inv.Issuer().DID().String(),
					"ability", content.RetrieveAbility,
					"space", space.String(),
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

				blob, err := retrievalService.Blobs().Get(ctx, digest, blobstore.WithRange(start, &end))
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						log.Debugw("blob not found", "status", http.StatusNotFound)
						notFoundErr := content.NewNotFoundError(fmt.Sprintf("blob not found: %s", digestStr))
						res := result.Error[content.RetrieveOk, failure.IPLDBuilderFailure](notFoundErr)
						resp := retrieval.NewResponse(http.StatusNotFound, nil, nil)
						return res, nil, resp, nil
					} else if errors.Is(err, blobstore.ErrRangeNotSatisfiable) {
						log.Debugw("range not satisfiable", "status", http.StatusRequestedRangeNotSatisfiable)
						rangeNotSatisfiableErr := content.NewRangeNotSatisfiableError(fmt.Sprintf("range not satisfiable: %d-%d", start, end))
						res := result.Error[content.RetrieveOk, failure.IPLDBuilderFailure](rangeNotSatisfiableErr)
						resp := retrieval.NewResponse(http.StatusRequestedRangeNotSatisfiable, nil, nil)
						return res, nil, resp, nil
					}
					log.Errorw("getting blob", "error", err)
					return nil, nil, retrieval.Response{}, fmt.Errorf("getting blob: %w", err)
				}

				res := result.Ok[content.RetrieveOk, failure.IPLDBuilderFailure](content.RetrieveOk{})
				status := http.StatusOK
				contentLength := end - start + 1
				headers := http.Header{}
				headers.Set("Content-Length", fmt.Sprintf("%d", contentLength))
				headers.Set("Content-Type", "application/octet-stream")
				headers.Set("Cache-Control", "public, max-age=29030400, immutable")
				headers.Set("Etag", fmt.Sprintf(`"%s"`, digestStr))
				headers.Set("Vary", "Accept-Encoding")
				if contentLength != uint64(blob.Size()) {
					status = http.StatusPartialContent
					headers.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, blob.Size()))
					headers.Add("Vary", "Range")
				}
				log.Debugw("serving bytes", "status", status, "size", contentLength)
				resp := retrieval.NewResponse(status, headers, blob.Body())
				return res, nil, resp, nil
			},
		),
	)
}
