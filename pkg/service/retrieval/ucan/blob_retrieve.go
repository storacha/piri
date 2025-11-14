package ucan

import (
	"context"
	"fmt"

	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/service/retrieval/handlers/spacecontent"
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
				if cap.With() != service.ID().DID().String() {
					return result.Error[blob.RetrieveOk, failure.IPLDBuilderFailure](blob.RetrieveError{
						ErrorName: InvalidResourceErrorName,
						Message:   fmt.Sprintf("resource is %s not %s", cap.With(), service.ID().DID()),
					}), nil, retrieval.Response{}, nil
				}
				// no range, pass nil for byteRange
				res, resp, err := spacecontent.Retrieve(ctx, service.Blobs(), inv, cap.Nb().Blob.Digest, nil)
				if err != nil {
					return nil, nil, retrieval.Response{}, err
				}
				return result.MapOk(res, func(o content.RetrieveOk) blob.RetrieveOk {
					return blob.RetrieveOk{}
				}), nil, resp, nil
			},
		),
	)
}
