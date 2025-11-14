package blobs

import (
	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/acceptancestore"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type BlobService struct {
	*options
}

func (b *BlobService) Access() access.Access {
	return b.access
}

func (b *BlobService) Allocations() allocationstore.AllocationStore {
	return b.allocStore
}

func (b *BlobService) Acceptances() acceptancestore.AcceptanceStore {
	return b.acceptStore
}

func (b *BlobService) Presigner() presigner.RequestPresigner {
	return b.presigner
}

func (b *BlobService) Store() blobstore.Blobstore {
	return b.blobStore
}

var _ Blobs = (*BlobService)(nil)

func New(opts ...Option) (*BlobService, error) {
	o := &options{}
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}

	return &BlobService{o}, nil
}
