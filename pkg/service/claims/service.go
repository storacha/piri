package claims

import (
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/principal"

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

func New(id principal.Signer, claimStore claimstore.ClaimStore, publisherStore store.PublisherStore, publicAddr multiaddr.Multiaddr, opts ...Option) (*ClaimService, error) {
	o := &options{}
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}

	publisher, err := publisher.New(
		id,
		publisherStore,
		publicAddr,
		publisher.WithDirectAnnounce(o.announceURLs...),
		publisher.WithIndexingService(o.indexingService),
		publisher.WithIndexingServiceProof(o.indexingServiceProofs...),
		publisher.WithAnnounceAddress(o.announceAddr),
		publisher.WithBlobAddress(o.blobAddr),
	)
	if err != nil {
		return nil, err
	}

	return &ClaimService{claimStore, publisher}, nil
}

func NewV2(
	claimStore claimstore.ClaimStore,
	publisher publisher.Publisher,
) *ClaimService {
	return &ClaimService{claimStore, publisher}
}
