package claims

import (
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/service/publisher"
	"github.com/storacha/piri/pkg/store/claimstore"
)

func NewService(
	claimStore claimstore.ClaimStore,
	pub publisher.Publisher,
) *claims.ClaimService {
	return claims.NewV2(claimStore, pub)
}
