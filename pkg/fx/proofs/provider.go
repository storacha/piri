package proofs

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/service/proofs"
)

var Module = fx.Module("proofs",
	fx.Provide(ProvideProofService),
)

func ProvideProofService() proofs.ProofService {
	return proofs.NewCachingProofService()
}
