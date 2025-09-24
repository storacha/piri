package retrieval

import (
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type RetrievalService struct {
	id          principal.Signer
	blobs       blobstore.BlobGetter
	allocations allocationstore.AllocationStore
}

func (r *RetrievalService) Allocations() allocationstore.AllocationStore {
	return r.allocations
}

func (r *RetrievalService) Blobs() blobstore.BlobGetter {
	return r.blobs
}

func (r *RetrievalService) ID() principal.Signer {
	return r.id
}

var _ Service = (*RetrievalService)(nil)

func New(id principal.Signer, blobs blobstore.BlobGetter, allocations allocationstore.AllocationStore) *RetrievalService {
	return &RetrievalService{id, blobs, allocations}
}
