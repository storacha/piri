package publisher

import (
	"fmt"

	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/service/publisher"
)

var Module = fx.Module("publisher",
	fx.Provide(
		NewService,
		// Also provide the interface
		fx.Annotate(
			func(svc *publisher.PublisherService) publisher.Publisher {
				return svc
			},
		),
		fx.Annotate(
			publisher.NewServer,
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

func NewService(
	cfg app.AppConfig,
	id principal.Signer,
	publisherStore store.PublisherStore,
) (*publisher.PublisherService, error) {
	pubCfg := cfg.Services.Publisher
	if pubCfg.PublicMaddr.String() == "" {
		return nil, fmt.Errorf("public address is required for publisher service")
	}

	return publisher.New(
		id,
		publisherStore,
		pubCfg.PublicMaddr,
		publisher.WithDirectAnnounce(pubCfg.AnnounceURLs...),
		publisher.WithIndexingService(cfg.External.IndexingService.Connection),
		publisher.WithIndexingServiceProof(cfg.External.IndexingService.Proofs...),
		publisher.WithAnnounceAddress(pubCfg.AnnounceMaddr),
		publisher.WithBlobAddress(pubCfg.BlobMaddr),
	)

}
