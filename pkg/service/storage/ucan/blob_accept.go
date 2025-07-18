package ucan

import (
	"context"

	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	blobhandler "github.com/storacha/piri/pkg/service/storage/handlers/blob"
)

type BlobAcceptService interface {
	ID() principal.Signer
	PDP() pdp.PDP
	Blobs() blobs.Blobs
	Claims() claims.Claims
}

func BlobAccept(storageService BlobAcceptService) server.Option {
	return server.WithServiceMethod(
		blob.AcceptAbility,
		server.Provide(
			blob.Accept,
			func(cap ucan.Capability[blob.AcceptCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (blob.AcceptOk, fx.Effects, error) {
				//
				// UCAN Validation
				//

				// only service principal can perform an allocation
				if cap.With() != iCtx.ID().DID().String() {
					return blob.AcceptOk{}, nil, NewUnsupportedCapabilityError(cap)
				}

				//
				// end UCAN Validation
				//

				// FIXME: use a real context, requires changes to server
				ctx := context.TODO()
				resp, err := blobhandler.Accept(ctx, storageService, &blobhandler.AcceptRequest{
					Space: cap.Nb().Space,
					Blob:  cap.Nb().Blob,
					Put:   cap.Nb().Put,
				})
				if err != nil {
					return blob.AcceptOk{}, nil, failure.FromError(err)
				}
				forks := []fx.Effect{fx.FromInvocation(resp.Claim)}
				res := blob.AcceptOk{
					Site: resp.Claim.Link(),
				}
				if resp.PDP != nil {
					forks = append(forks, fx.FromInvocation(resp.PDP))
					tmp := resp.PDP.Link()
					res.PDP = &tmp
				}

				return res, fx.NewEffects(fx.WithFork(forks...)), nil
			},
		),
	)
}
