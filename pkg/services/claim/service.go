package claim

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/services/types"
	"github.com/storacha/piri/pkg/store/claimstore"
)

// Service provides claim storage and publishing functionality
type Service struct {
	store     claimstore.ClaimStore
	publisher types.Publisher
}

// Params defines the dependencies for Service
type Params struct {
	fx.In
	Store     claimstore.ClaimStore
	Publisher types.Publisher
}

// NewService creates a new claim service
func NewService(params Params) *Service {
	return &Service{
		store:     params.Store,
		publisher: params.Publisher,
	}
}

// Store returns the underlying claim store
func (s *Service) Store() claimstore.ClaimStore {
	return s.store
}

// Publisher returns the claim publisher
func (s *Service) Publisher() types.Publisher {
	return s.publisher
}
