package ucan

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/blobstore"
)

// InvalidResourceErrorName is the name given to an error where the resource did
// not match the service DID.
const InvalidResourceErrorName = "InvalidResource"

type BlobRetrievalService interface {
	ID() principal.Signer
	Blobs() blobstore.BlobGetter
}

func BlobRetrieve(service BlobRetrievalService) retrieval.Option {
	return retrieval.WithServiceMethod(
		blob.RetrieveAbility,
		retrieval.Provide(
			blob.Retrieve,
			func(ctx context.Context, cap ucan.Capability[blob.RetrieveCaveats], inv invocation.Invocation, iCtx server.InvocationContext, request retrieval.Request) (result.Result[blob.RetrieveOk, failure.IPLDBuilderFailure], fx.Effects, retrieval.Response, error) {
				resource, err := did.Parse(cap.With())
				if err != nil {
					return nil, nil, retrieval.Response{}, fmt.Errorf("parsing resource DID: %w", err)
				}
				if resource != service.ID().DID() {
					return result.Error[blob.RetrieveOk, failure.IPLDBuilderFailure](blob.RetrieveError{
						ErrorName: InvalidResourceErrorName,
						Message:   fmt.Sprintf("resource is %s not %s", resource, service.ID().DID()),
					}), nil, retrieval.Response{}, nil
				}

				nb := cap.Nb()
				digest := nb.Blob.Digest
				digestStr := digestutil.Format(digest)

				log := log.With(
					"client", inv.Issuer().DID().String(),
					"ability", blob.RetrieveAbility,
					"digest", digestStr,
				)

				obj, err := service.Blobs().Get(ctx, digest)
				if err != nil {
					if errors.Is(err, store.ErrNotFound) {
						log.Debugw("blob not found", "status", http.StatusNotFound)
						notFoundErr := content.NewNotFoundError(fmt.Sprintf("blob not found: %s", digestStr))
						res := result.Error[blob.RetrieveOk, failure.IPLDBuilderFailure](notFoundErr)
						resp := retrieval.NewResponse(http.StatusNotFound, nil, nil)
						return res, nil, resp, nil
					}
					log.Errorw("getting blob", "error", err)
					return nil, nil, retrieval.Response{}, fmt.Errorf("getting blob: %w", err)
				}

				res := result.Ok[blob.RetrieveOk, failure.IPLDBuilderFailure](blob.RetrieveOk{})
				status := http.StatusOK
				contentLength := obj.Size()
				headers := http.Header{}
				headers.Set("Content-Length", fmt.Sprintf("%d", contentLength))
				headers.Set("Content-Type", "application/octet-stream")
				headers.Set("Cache-Control", "public, max-age=29030400, immutable")
				headers.Set("Etag", fmt.Sprintf(`"%s"`, digestStr))
				headers.Set("Vary", "Accept-Encoding")
				log.Debugw("serving bytes", "status", status, "size", contentLength)
				resp := retrieval.NewResponse(status, headers, obj.Body())
				return res, nil, resp, nil
			},
		),
	)
}
