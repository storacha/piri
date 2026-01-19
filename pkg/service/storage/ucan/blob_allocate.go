package ucan

import (
	"context"

	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	blobhandler "github.com/storacha/piri/pkg/service/storage/handlers/blob"
)

const maxUploadSize = 127 * (1 << 25)

type BlobAllocateService interface {
	PDP() pdp.PDP
	Blobs() blobs.Blobs
}

func WithBlobAllocateMethod(storageService BlobAllocateService) server.Option {
	return server.WithServiceMethod(
		blob.AllocateAbility,
		server.Provide(
			blob.Allocate,
			func(ctx context.Context, cap ucan.Capability[blob.AllocateCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (result.Result[blob.AllocateOk, failure.IPLDBuilderFailure], fx.Effects, error) {
				//
				// UCAN Validation
				//

				// only service principal can perform an allocation
				if cap.With() != iCtx.ID().DID().String() {
					return result.Error[blob.AllocateOk, failure.IPLDBuilderFailure](NewUnsupportedCapabilityError(cap)), nil, nil
				}

				// enforce max upload size requirements
				if cap.Nb().Blob.Size > maxUploadSize {
					return result.Error[blob.AllocateOk, failure.IPLDBuilderFailure](NewBlobSizeLimitExceededError(cap.Nb().Blob.Size, maxUploadSize)), nil, nil
				}

				//
				// end UCAN Validation
				//

				resp, err := blobhandler.Allocate(ctx, storageService, &blobhandler.AllocateRequest{
					Space: cap.Nb().Space,
					Blob:  cap.Nb().Blob,
					Cause: inv.Link(),
				})
				if err != nil {
					return nil, nil, err
				}

				return result.Ok[blob.AllocateOk, failure.IPLDBuilderFailure](
					blob.AllocateOk{
						Size:    resp.Size,
						Address: resp.Address,
					},
				), nil, nil
			},
		),
	)
}
