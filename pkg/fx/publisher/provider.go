package publisher

import (
	"fmt"

	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/service/publisher"
)

var Module = fx.Module("publisher",
	fx.Provide(
		// Also provide the interface
		fx.Annotate(
			NewService,
			fx.As(new(publisher.Publisher)),
		),
		fx.Annotate(
			publisher.NewServer,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

func NewService(
	cfg app.AppConfig,
	id principal.Signer,
	publisherStore store.PublisherStore,
) (*publisher.PublisherService, error) {
	pubCfg := cfg.UCANService.Services.Publisher
	if pubCfg.PublicMaddr.String() == "" {
		return nil, fmt.Errorf("public address is required for publisher service")
	}

	return publisher.New(
		id,
		publisherStore,
		pubCfg.PublicMaddr,
		publisher.WithDirectAnnounce(pubCfg.AnnounceURLs...),
		publisher.WithIndexingService(cfg.UCANService.Services.Indexer.Connection),
		publisher.WithIndexingServiceProof(cfg.UCANService.Services.Indexer.Proofs...),
		publisher.WithAnnounceAddress(pubCfg.AnnounceMaddr),
		publisher.WithBlobAddress(pubCfg.BlobMaddr),
	)

}
