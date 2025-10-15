package ucan

import (
	"context"
	"fmt"
	"io"

	"github.com/storacha/go-libstoracha/capabilities/access"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/go-ucanto/validator"
)

type AccessGrantService interface {
	ID() principal.Signer
	ValidationContext() validator.ClaimContext
}

func AccessGrant(storageService AccessGrantService) server.Option {
	return server.WithServiceMethod(
		access.GrantAbility,
		server.Provide(
			access.Grant,
			func(ctx context.Context, cap ucan.Capability[access.GrantCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (result.Result[access.GrantOk, failure.IPLDBuilderFailure], fx.Effects, error) {
				var cause invocation.Invocation
				if cap.Nb().Cause != nil {
					bs, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(inv.Blocks()))
					if err != nil {
						return nil, nil, fmt.Errorf("reading invocation blocks: %w", err)
					}
					i, err := invocation.NewInvocationView(cap.Nb().Cause, bs)
					if err != nil {
						return nil, nil, fmt.Errorf("creating cause delegation: %w", err)
					}
					cause = i
				}

				delegations := map[string]delegation.Delegation{}
				for _, cap := range cap.Nb().Att {
					res, err := grantAbility(ctx, storageService.ValidationContext(), storageService.ID(), cap.Can, cause)
					if err != nil {
						return nil, nil, err
					}
					o, x := result.Unwrap(res)
					if x != (access.GrantError{}) {
						return result.Error[access.GrantOk, failure.IPLDBuilderFailure](x), nil, nil
					}
					delegations[o.Link().String()] = o
				}

				grant := access.GrantOk{
					Delegations: access.DelegationsModel{Values: map[string][]byte{}},
				}
				for cid, dlg := range delegations {
					r := dlg.Archive()
					b, err := io.ReadAll(r)
					if err != nil {
						return nil, nil, fmt.Errorf("reading granted delegation archive: %w", err)
					}
					grant.Delegations.Keys = append(grant.Delegations.Keys, cid)
					grant.Delegations.Values[cid] = b
				}

				return result.Ok[access.GrantOk, failure.IPLDBuilderFailure](grant), nil, nil
			},
		),
	)
}

func grantAbility(ctx context.Context, claimctx validator.ClaimContext, id principal.Signer, ability ucan.Ability, cause invocation.Invocation) (result.Result[delegation.Delegation, access.GrantError], error) {
	switch ability {
	case blob.RetrieveAbility:
		return grantBlobRetrieve(ctx, claimctx, id, cause)
	default:
		return result.Error[delegation.Delegation](
			access.GrantError{
				ErrorName: "UnknownAbility",
				Message:   fmt.Sprintf("unknown ability: %s", ability),
			},
		), nil
	}
}

func grantBlobRetrieve(ctx context.Context, claimctx validator.ClaimContext, id principal.Signer, cause invocation.Invocation) (result.Result[delegation.Delegation, access.GrantError], error) {
	// Grant blob retrieve for the following:
	// 1. if the cause is an `assert/index` issued by an indexing service
	// 2. if the cause is a `blob/replica/allocate` issued by an upload service
	if cause == nil {
		return result.Error[delegation.Delegation, access.GrantError](access.GrantError{
			ErrorName: "MissingCause",
			Message:   "grant requires supporting contextual invocation",
		}), nil
	}

	if len(cause.Capabilities()) == 0 {
		return result.Error[delegation.Delegation, access.GrantError](access.GrantError{
			ErrorName: "InvalidCause",
			Message:   "invalid cause invocation: missing capabilities",
		}), nil
	}

	var validationErr validator.Unauthorized
	switch cause.Capabilities()[0].Can() {
	case replica.AllocateAbility:
		vctx := validator.NewValidationContext(
			id.Verifier(),
			replica.Allocate,
			claimctx.CanIssue,
			claimctx.ValidateAuthorization,
			claimctx.ResolveProof,
			claimctx.ParsePrincipal,
			claimctx.ResolveDIDKey,
			claimctx.AuthorityProofs()...,
		)
		_, validationErr = validator.Access(ctx, cause, vctx)
	case assert.IndexAbility:
	default:
		return result.Error[delegation.Delegation, access.GrantError](access.GrantError{
			ErrorName: "UnknownCause",
			Message:   "unknown cause invocation",
		}), nil
	}

	if validationErr != nil {
		return result.Error[delegation.Delegation, access.GrantError](access.GrantError{
			ErrorName: validationErr.Name(),
			Message:   validationErr.Error(),
		}), nil
	}
}

func validateAllocateInvocation(ctx context.Context, inv invocation.Invocation) error {

}
