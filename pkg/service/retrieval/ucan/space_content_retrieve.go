package ucan

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/ucan"
	"golang.org/x/sync/errgroup"

	"github.com/storacha/piri/pkg/internal/digestutil"
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
			func(ctx context.Context, cap ucan.Capability[content.RetrieveCaveats], inv invocation.Invocation, iCtx server.InvocationContext, request retrieval.Request) (result.Result[content.RetrieveCaveats, failure.IPLDBuilderFailure], fx.Effects, retrieval.Response, error) {
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
					"space", space.String(),
					"digest", digestStr,
					"range", fmt.Sprintf("%d-%d", start, end),
				)

				var blob blobstore.Object
				g, gctx := errgroup.WithContext(ctx)
				g.Go(func() error {
					_, err = retrievalService.Allocations().Get(gctx, digest, space)
					return err
				})
				g.Go(func() error {
					if end < start {
						return blobstore.ErrRangeNotSatisfiable
					}
					length := end - start + 1
					byteRange := blobstore.Range{Offset: start, Length: &length}
					blob, err = retrievalService.Blobs().Get(gctx, digest, blobstore.WithRange(byteRange))
					return err
				})
				if err := g.Wait(); err != nil {
					if errors.Is(err, store.ErrNotFound) {
						log.Debugw("not found", "status", http.StatusNotFound)
						notFoundErr := content.NewNotFoundError(fmt.Sprintf("blob not found: %s", digestStr))
						res := result.Error[content.RetrieveCaveats, failure.IPLDBuilderFailure](notFoundErr)
						resp := retrieval.NewResponse(http.StatusNotFound, nil, nil)
						return res, nil, resp, nil
					}
					if errors.Is(err, blobstore.ErrRangeNotSatisfiable) {
						log.Debugw("range not satisfiable", "status", http.StatusRequestedRangeNotSatisfiable)
						rangeNotSatisfiableErr := content.NewRangeNotSatisfiableError(fmt.Sprintf("range not satisfiable: %d-%d", start, end))
						res := result.Error[content.RetrieveCaveats, failure.IPLDBuilderFailure](rangeNotSatisfiableErr)
						resp := retrieval.NewResponse(http.StatusRequestedRangeNotSatisfiable, nil, nil)
						return res, nil, resp, nil
					}
					log.Errorw("getting allocation and blob", "error", err)
					return nil, nil, retrieval.Response{}, fmt.Errorf("getting allocation and blob: %w", err)
				}

				res := result.Ok[content.RetrieveCaveats, failure.IPLDBuilderFailure](content.RetrieveCaveats{})
				status := http.StatusOK
				contentLength := end - start + 1
				headers := http.Header{}
				headers.Add("Content-Length", fmt.Sprintf("%d", contentLength))
				headers.Add("Content-Type", "application/octet-stream")
				headers.Add("Cache-Control", "public, max-age=29030400, immutable")
				headers.Add("Etag", fmt.Sprintf(`"%s"`, digestStr))
				headers.Add("Vary", "Range, Accept-Encoding")
				if contentLength != uint64(blob.Size()) {
					status = http.StatusPartialContent
					headers.Add("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, blob.Size()))
				}
				log.Debugw("serving bytes", "status", status, "size", contentLength)
				resp := retrieval.NewResponse(status, headers, blob.Body())
				return res, nil, resp, nil
			},
		),
	)
}
