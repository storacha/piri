package publisher

import (
	"fmt"

	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/service/publisher"
)

func NewService(
	cfg app.PublisherServiceConfig,
	id principal.Signer,
	publisherStore store.PublisherStore,
) (*publisher.PublisherService, error) {
	if cfg.PublicMaddr.String() == "" {
		return nil, fmt.Errorf("public address is required for publisher service")
	}

	return publisher.New(
		id,
		publisherStore,
		cfg.PublicMaddr,
		publisher.WithDirectAnnounce(cfg.AnnounceURLs...),
		publisher.WithIndexingService(cfg.IndexingService),
		publisher.WithIndexingServiceProof(cfg.IndexingServiceProofs...),
		publisher.WithAnnounceAddress(cfg.AnnounceMaddr),
		publisher.WithBlobAddress(cfg.BlobMaddr),
	)

}
