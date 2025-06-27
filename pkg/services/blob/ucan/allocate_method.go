package ucan

import (
	"context"

	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-ucanto/core/invocation"
	fx2 "github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/go-ucanto/validator"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/services/errors"
	"github.com/storacha/piri/pkg/services/types"
)

// TODO this can be made into a configuration option
const maxUploadSize = 127 * (1 << 25)

// AllocateMethod handles blob/allocate capability
type AllocateMethod struct {
	id          principal.Signer
	blobService types.Blobs
	pdpService  pdp.PDP
}

func (h *AllocateMethod) PDP() pdp.PDP {
	return h.pdpService
}

func (h *AllocateMethod) Blobs() types.Blobs {
	return h.blobService
}

// AllocateParams defines dependencies for the handler
type AllocateParams struct {
	fx.In
	ID          principal.Signer
	BlobService types.Blobs
	PDPService  pdp.PDP `optional:"true"`
}

// NewAllocate creates a new allocate handler
func NewAllocate(params AllocateParams) *AllocateMethod {
	return &AllocateMethod{
		id:          params.ID,
		blobService: params.BlobService,
		pdpService:  params.PDPService,
	}
}

// Option returns the server option for this handler
func (h *AllocateMethod) Option() server.Option {
	return server.WithServiceMethod(
		blob.AllocateAbility,
		server.Provide(h.Provide()),
	)
}

// Holy generics batman!

// Provide returns the capability parser and handler function
func (h *AllocateMethod) Provide() (
	validator.CapabilityParser[blob.AllocateCaveats],
	server.HandlerFunc[blob.AllocateCaveats, blob.AllocateOk],
) {
	handler := func(
		c ucan.Capability[blob.AllocateCaveats],
		i invocation.Invocation,
		ictx server.InvocationContext,
	) (blob.AllocateOk, fx2.Effects, error) {
		//
		// UCAN Validation
		//

		// only service principal can perform an allocation
		if c.With() != ictx.ID().DID().String() {
			return blob.AllocateOk{}, nil, errors.NewUnsupportedCapabilityError(c)
		}

		// enforce max upload size requirements
		if c.Nb().Blob.Size > maxUploadSize {
			return blob.AllocateOk{}, nil, errors.NewBlobSizeLimitExceededError(c.Nb().Blob.Size, maxUploadSize)
		}

		//
		// end UCAN Validation
		//

		// FIXME: use a real context, requires changes to server
		ctx := context.TODO()
		resp, err := Allocate(ctx, h, &AllocateRequest{
			Space: c.Nb().Space,
			Blob:  c.Nb().Blob,
			Cause: i.Link(),
		})
		if err != nil {
			return blob.AllocateOk{}, nil, failure.FromError(err)
		}

		return blob.AllocateOk{
			Size:    resp.Size,
			Address: resp.Address,
		}, nil, nil
	}

	return blob.Allocate, handler
}
