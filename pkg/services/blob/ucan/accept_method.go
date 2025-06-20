package ucan

import (
	"context"

	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-ucanto/core/invocation"
	ucanfx "github.com/storacha/go-ucanto/core/receipt/fx"
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

type AcceptMethod struct {
	id           principal.Signer
	blobService  types.Blobs
	pdpService   pdp.PDP
	claimService types.Claims
}

func (h *AcceptMethod) ID() principal.Signer {
	return h.id
}

func (h *AcceptMethod) PDP() pdp.PDP {
	return h.pdpService
}

func (h *AcceptMethod) Blobs() types.Blobs {
	return h.blobService
}

func (h *AcceptMethod) Claims() types.Claims {
	return h.claimService
}

type AcceptParams struct {
	fx.In
	ID           principal.Signer
	BlobService  types.Blobs
	PDPService   pdp.PDP
	ClaimService types.Claims
}

func NewAccept(params AcceptParams) *AcceptMethod {
	return &AcceptMethod{
		id:           params.ID,
		blobService:  params.BlobService,
		pdpService:   params.PDPService,
		claimService: params.ClaimService,
	}
}

// Option returns the server option for this handler
func (h *AcceptMethod) Option() server.Option {
	return server.WithServiceMethod(
		blob.AcceptAbility,
		server.Provide(h.Provide()),
	)
}

func (h *AcceptMethod) Provide() (
	validator.CapabilityParser[blob.AcceptCaveats],
	server.HandlerFunc[blob.AcceptCaveats, blob.AcceptOk],
) {
	handler := func(
		c ucan.Capability[blob.AcceptCaveats],
		i invocation.Invocation,
		ictx server.InvocationContext,
	) (blob.AcceptOk, ucanfx.Effects, error) {
		//
		// UCAN Validation
		//

		// only service principal can perform an allocation
		if c.With() != ictx.ID().DID().String() {
			return blob.AcceptOk{}, nil, errors.NewUnsupportedCapabilityError(c)
		}

		//
		// end UCAN Validation
		//

		// FIXME: use a real context, requires changes to server
		ctx := context.TODO()
		resp, err := Accept(ctx, h, &AcceptRequest{
			Space: c.Nb().Space,
			Blob:  c.Nb().Blob,
			Put:   c.Nb().Put,
		})
		if err != nil {
			return blob.AcceptOk{}, nil, failure.FromError(err)
		}
		forks := []ucanfx.Effect{ucanfx.FromInvocation(resp.Claim)}
		res := blob.AcceptOk{
			Site: resp.Claim.Link(),
		}
		if resp.PDP != nil {
			forks = append(forks, ucanfx.FromInvocation(resp.PDP))
			tmp := resp.PDP.Link()
			res.PDP = &tmp
		}

		return res, ucanfx.NewEffects(ucanfx.WithFork(forks...)), nil
	}

	return blob.Accept, handler
}
