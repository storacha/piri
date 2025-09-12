package ucan

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type SpaceContentRetrieveService interface {
	Blobs() blobs.Blobs
}

func SpaceContentRetrieve(storageService SpaceContentRetrieveService) retrieval.Option {
	return retrieval.WithServiceMethod(
		content.RetrieveAbility,
		retrieval.Provide(
			content.Retrieve,
			func(ctx context.Context, cap ucan.Capability[content.RetrieveCaveats], inv invocation.Invocation, iCtx server.InvocationContext, request retrieval.Request) (result.Result[content.RetrieveCaveats, failure.IPLDBuilderFailure], fx.Effects, retrieval.Response, error) {
				space, err := did.Parse(cap.With())
				if err != nil {
					return nil, nil, retrieval.Response{}, err
				}

				length := uint64(cap.Nb().Range.End - cap.Nb().Range.Start + 1)
				byteRange := blobstore.Range{
					Offset: cap.Nb().Range.Start,
					Length: &length,
				}
				blob, err := storageService.Blobs().Store().Get(ctx, cap.Nb().Blob.Digest, blobstore.WithRange(byteRange))
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						notFoundErr := content.NewNotFoundError(fmt.Sprintf("blob not found: %s", digestutil.Format(cap.Nb().Blob.Digest)))
						res := result.Error[content.RetrieveCaveats, failure.IPLDBuilderFailure](notFoundErr)
						resp := retrieval.NewResponse(http.StatusNotFound, nil, nil)
						return res, nil, resp, nil
					}
					return nil, nil, retrieval.Response{}, err
				}

				// ensure we have an allocation for this blob
				allocs, err := storageService.Blobs().Allocations().ListBySpace(ctx, cap.Nb().Blob.Digest, space)
				if err != nil {
					return nil, nil, retrieval.Response{}, err
				}
				if len(allocs) == 0 {
					notFoundErr := content.NewNotFoundError(fmt.Sprintf("blob not found: %s", digestutil.Format(cap.Nb().Blob.Digest)))
					res := result.Error[content.RetrieveCaveats, failure.IPLDBuilderFailure](notFoundErr)
					resp := retrieval.NewResponse(http.StatusNotFound, nil, nil)
					return res, nil, resp, nil
				}

				res := result.Ok[content.RetrieveCaveats, failure.IPLDBuilderFailure](content.RetrieveCaveats{})
				status := http.StatusOK
				headers := http.Header{}
				headers.Add("Content-Length", fmt.Sprintf("%d", length))
				headers.Add("Content-Type", "application/octet-stream")
				headers.Add("Cache-Control", "public, max-age=29030400, immutable")
				headers.Add("Etag", fmt.Sprintf(`"%s"`, digestutil.Format(cap.Nb().Blob.Digest)))
				headers.Add("Vary", "Range, Accept-Encoding")
				if length != uint64(blob.Size()) {
					status = http.StatusPartialContent
					headers.Add("Content-Range", fmt.Sprintf("bytes %d-%d/%d", cap.Nb().Range.Start, cap.Nb().Range.End, blob.Size()))
				}
				resp := retrieval.NewResponse(status, headers, blob.Body())
				return res, nil, resp, nil
			},
		),
	)
}
