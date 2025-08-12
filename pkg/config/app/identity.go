package app

import (
	"github.com/storacha/go-ucanto/principal"
)

// IdentityConfig contains identity-related configuration
type IdentityConfig struct {
	// The principal signer for this service
	Signer principal.Signer
}
