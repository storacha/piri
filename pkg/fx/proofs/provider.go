package proofs

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/service/proofs"
)

var Module = fx.Module("proofs",
	fx.Provide(ProvideProofService),
)

func ProvideProofService(cfg app.IdentityConfig) proofs.ProofService {
	return proofs.NewCachingProofService(cfg.Signer)
}
