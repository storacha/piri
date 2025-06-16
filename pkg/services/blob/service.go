package blob

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

// Service provides blob storage functionality
type Service struct {
	store       blobstore.Blobstore
	allocations allocationstore.AllocationStore
	presigner   presigner.RequestPresigner
	access      access.Access
}

// Params defines the dependencies for Service
type Params struct {
	fx.In
	Store       blobstore.Blobstore
	Allocations allocationstore.AllocationStore
	Presigner   presigner.RequestPresigner
	Access      access.Access
}

// NewService creates a new blob service
func NewService(params Params) *Service {
	return &Service{
		store:       params.Store,
		allocations: params.Allocations,
		presigner:   params.Presigner,
		access:      params.Access,
	}
}

// Store returns the underlying blob store
func (s *Service) Store() blobstore.Blobstore {
	return s.store
}

// Allocations returns the allocation store
func (s *Service) Allocations() allocationstore.AllocationStore {
	return s.allocations
}

// Presigner returns the request presigner
func (s *Service) Presigner() presigner.RequestPresigner {
	return s.presigner
}

// Access returns the access controller
func (s *Service) Access() access.Access {
	return s.access
}
