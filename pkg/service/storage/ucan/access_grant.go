package ucan

import (
	"context"
	"fmt"
	"io"
	"time"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/access"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/client"
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

const (
	// UnknownAbilityErrorName is the name given to an error where the ability
	// requested to be granted is unknown to the service.
	UnknownAbilityErrorName = "UnknownAbility"
	// UnknownCauseErrorName is the name given to an error where the cause
	// invocation sent as context for the delegation is not recognised.
	UnknownCauseErrorName = "UnknownCause"
	// MissingCauseErrorName is the name given to an error where a required cause
	// invocation has not been provided in the invocation to request a grant.
	MissingCauseErrorName = "MissingCause"
	// InvalidCauseErrorName is the name given to an error where the cause
	// invocation has been determined to be invalid is some way. See the error
	// message for details.
	InvalidCauseErrorName = "InvalidCause"
	// UnauthorizedCauseErrorName is the name given to an error where the cause
	// invocation failed UCAN validation.
	UnauthorizedCauseErrorName = "UnauthorizedCause"
)

// validity is the time a granted delegation is valid for.
const validity = time.Hour

var (
	errUnknownCause = access.GrantError{
		ErrorName: UnknownCauseErrorName,
		Message:   "unknown cause invocation",
	}
	errMissingCause = access.GrantError{
		ErrorName: MissingCauseErrorName,
		Message:   "grant requires supporting contextual invocation",
	}
)

type AccessGrantService interface {
	ID() principal.Signer
	ClaimValidationContext() validator.ClaimContext
	UploadConnection() client.Connection
}

func AccessGrant(service AccessGrantService) server.Option {
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
						return nil, nil, fmt.Errorf("creating cause invocation: %w", err)
					}
					cause = i
				}

				delegations := map[string]delegation.Delegation{}
				for _, cap := range cap.Nb().Att {
					res, err := grantCapability(ctx, service, inv.Issuer(), cap.Can, cause)
					if err != nil {
						return nil, nil, err
					}
					o, x := result.Unwrap(res)
					if x != nil {
						return result.Error[access.GrantOk](x), nil, nil
					}
					delegations[o.Link().String()] = o
				}

				res := access.GrantOk{
					Delegations: access.DelegationsModel{Values: map[string][]byte{}},
				}
				for cid, dlg := range delegations {
					r := dlg.Archive()
					b, err := io.ReadAll(r)
					if err != nil {
						return nil, nil, fmt.Errorf("reading granted delegation archive: %w", err)
					}
					res.Delegations.Keys = append(res.Delegations.Keys, cid)
					res.Delegations.Values[cid] = b
				}

				return result.Ok[access.GrantOk, failure.IPLDBuilderFailure](res), nil, nil
			},
		),
	)
}

func grantCapability(
	ctx context.Context,
	service AccessGrantService,
	audience ucan.Principal,
	ability ucan.Ability,
	cause invocation.Invocation,
) (result.Result[delegation.Delegation, failure.IPLDBuilderFailure], error) {
	switch ability {
	case blob.RetrieveAbility:
		return grantBlobRetrieve(ctx, service, audience, cause)
	default:
		return result.Error[delegation.Delegation, failure.IPLDBuilderFailure](newUnknownAbilityError(ability)), nil
	}
}

// Grant blob retrieve for the following:
// 1. if the cause is a `blob/replica/allocate` issued by an upload service
// 2. if the cause is an `assert/index` issued by an upload service
func grantBlobRetrieve(
	ctx context.Context,
	service AccessGrantService,
	audience ucan.Principal,
	cause invocation.Invocation,
) (result.Result[delegation.Delegation, failure.IPLDBuilderFailure], error) {
	if cause == nil {
		return result.Error[delegation.Delegation, failure.IPLDBuilderFailure](errMissingCause), nil
	}
	if len(cause.Capabilities()) == 0 {
		err := newInvalidCauseError("missing capabilities")
		return result.Error[delegation.Delegation, failure.IPLDBuilderFailure](err), nil
	}
	if cause.Audience().DID() != audience.DID() {
		err := newInvalidCauseError(fmt.Sprintf("audience is %s not %s", cause.Audience().DID(), audience.DID()))
		return result.Error[delegation.Delegation, failure.IPLDBuilderFailure](err), nil
	}
	if cause.Issuer().DID() != service.UploadConnection().ID().DID() {
		err := newInvalidCauseError(fmt.Sprintf("issuer is %s not %s", cause.Issuer().DID(), service.UploadConnection().ID().DID()))
		return result.Error[delegation.Delegation, failure.IPLDBuilderFailure](err), nil
	}

	causeAbility := cause.Capabilities()[0].Can()
	var digest multihash.Multihash
	switch causeAbility {
	case replica.AllocateAbility:
		vctx := newValidationContextFromClaimContext(replica.Allocate, service.ClaimValidationContext())
		auth, verr := validator.Access(ctx, cause, vctx)
		if verr != nil {
			return result.Error[delegation.Delegation, failure.IPLDBuilderFailure](newUnauthorizedCauseError(verr)), nil
		}
		digest = auth.Capability().Nb().Blob.Digest
	case assert.IndexAbility:
		vctx := newValidationContextFromClaimContext(assert.Index, service.ClaimValidationContext())
		auth, verr := validator.Access(ctx, cause, vctx)
		if verr != nil {
			return result.Error[delegation.Delegation, failure.IPLDBuilderFailure](newUnauthorizedCauseError(verr)), nil
		}
		digest = auth.Capability().Nb().Index.(cidlink.Link).Hash()
	default:
		return result.Error[delegation.Delegation, failure.IPLDBuilderFailure](errUnknownCause), nil
	}

	d, err := blob.Retrieve.Delegate(
		service.ID(),
		audience,
		// Allow blob retrieval on "us" i.e. this agent is the resource.
		service.ID().DID().String(),
		// Allow only retrieval of the specified blob
		blob.RetrieveCaveats{Blob: blob.Blob{Digest: digest}},
		delegation.WithExpiration(ucan.Now()+int(validity.Seconds())),
	)
	if err != nil {
		return nil, err
	}

	log.Infow("delegated capability", "ability", blob.RetrieveAbility, "digest", digestutil.Format(digest), "audience", audience.DID().String(), "cause", causeAbility)
	return result.Ok[delegation.Delegation, failure.IPLDBuilderFailure](d), nil
}

func newValidationContextFromClaimContext[T any](
	capability validator.CapabilityParser[T],
	ctx validator.ClaimContext,
) validator.ValidationContext[T] {
	return validator.NewValidationContext(
		ctx.Authority(),
		capability,
		ctx.CanIssue,
		ctx.ValidateAuthorization,
		ctx.ResolveProof,
		ctx.ParsePrincipal,
		ctx.ResolveDIDKey,
		ctx.AuthorityProofs()...,
	)
}

func newUnknownAbilityError(ability string) access.GrantError {
	return access.GrantError{
		ErrorName: UnknownAbilityErrorName,
		Message:   fmt.Sprintf("unknown ability: %s", ability),
	}
}

func newInvalidCauseError(msg string) access.GrantError {
	return access.GrantError{
		ErrorName: InvalidCauseErrorName,
		Message:   fmt.Sprintf("invalid cause invocation: %s", msg),
	}
}

func newUnauthorizedCauseError(err validator.Unauthorized) access.GrantError {
	return access.GrantError{ErrorName: UnauthorizedCauseErrorName, Message: err.Error()}
}
