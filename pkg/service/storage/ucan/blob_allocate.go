package ucan

import (
	"context"

	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
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

func BlobAllocate(storageService BlobAllocateService) server.Option {
	return server.WithServiceMethod(
		blob.AllocateAbility,
		server.Provide(
			blob.Allocate,
			func(cap ucan.Capability[blob.AllocateCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (blob.AllocateOk, fx.Effects, error) {
				//
				// UCAN Validation
				//

				// only service principal can perform an allocation
				if cap.With() != iCtx.ID().DID().String() {
					return blob.AllocateOk{}, nil, NewUnsupportedCapabilityError(cap)
				}

				// enforce max upload size requirements
				if cap.Nb().Blob.Size > maxUploadSize {
					return blob.AllocateOk{}, nil, NewBlobSizeLimitExceededError(cap.Nb().Blob.Size, maxUploadSize)
				}

				//
				// end UCAN Validation
				//

				// FIXME: use a real context, requires changes to server
				ctx := context.TODO()
				resp, err := blobhandler.Allocate(ctx, storageService, &blobhandler.AllocateRequest{
					Space: cap.Nb().Space,
					Blob:  cap.Nb().Blob,
					Cause: inv.Link(),
				})
				if err != nil {
					return blob.AllocateOk{}, nil, failure.FromError(err)
				}

				return blob.AllocateOk{
					Size:    resp.Size,
					Address: resp.Address,
				}, nil, nil
			},
		),
	)
}
