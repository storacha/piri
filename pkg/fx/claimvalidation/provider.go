package claimvalidation

import (
	"context"

	"go.uber.org/fx"

	"github.com/storacha/go-ucanto/principal"
	edverifier "github.com/storacha/go-ucanto/principal/ed25519/verifier"
	"github.com/storacha/go-ucanto/validator"
)

var Module = fx.Module("claimvalidation",
	fx.Provide(
		NewClaimValidationContext,
	),
)

func NewClaimValidationContext(id principal.Signer, resolver validator.PrincipalResolver) validator.ClaimContext {
	return validator.NewClaimContext(
		id.Verifier(),
		validator.IsSelfIssued,
		func(context.Context, validator.Authorization[any]) validator.Revoked {
			return nil
		},
		validator.ProofUnavailable,
		edverifier.Parse,
		resolver.ResolveDIDKey,
		validator.NotExpiredNotTooEarly,
	)
}
