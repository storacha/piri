package ucan

import (
	"context"

	"github.com/storacha/go-libstoracha/capabilities/access"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"

	blobhandler "github.com/storacha/piri/pkg/service/storage/handlers/blob"
)

type AccessGrantService interface {
	ID() principal.Signer
}

func AccessGrant(storageService AccessGrantService) server.Option {
	return server.WithServiceMethod(
		access.GrantAbility,
		server.Provide(
			access.Grant,
			func(ctx context.Context, cap ucan.Capability[access.GrantCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (result.Result[access.GrantOk, failure.IPLDBuilderFailure], fx.Effects, error) {
				var cause delegation.Delegation
				if cap.Nb().Cause != nil {
					blocks
					cause := delegation.NewDelegationView(cap.Nb().Cause)
				}

				delegations := map[string]delegation.Delegation{}
				for _, cap := range cap.Nb().Att {
					// TODO: use blob.RetrieveAbility
					if cap.Can != "blob/retrieve" {
						continue
					}

				}

				delegationsModel := access.DelegationsModel{
					Values: map[string][]byte{},
				}

				resp, err := blobhandler.Accept(ctx, storageService, &blobhandler.AcceptRequest{
					Space: cap.Nb().Space,
					Blob:  cap.Nb().Blob,
					Put:   cap.Nb().Put,
				})
				if err != nil {
					return nil, nil, err
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

				return result.Ok[access.GrantOk, failure.IPLDBuilderFailure](res), nil, nil
			},
		),
	)
}
