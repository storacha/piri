package claims

import (
	"github.com/storacha/piri/pkg/service/publisher"
	"github.com/storacha/piri/pkg/store/claimstore"
)

type ClaimService struct {
	store     claimstore.ClaimStore
	publisher publisher.Publisher
}

func (c *ClaimService) Publisher() publisher.Publisher {
	return c.publisher
}

func (c *ClaimService) Store() claimstore.ClaimStore {
	return c.store
}

var _ Claims = (*ClaimService)(nil)

func New(
	claimStore claimstore.ClaimStore,
	publisher publisher.Publisher,
) *ClaimService {
	return &ClaimService{claimStore, publisher}
}
