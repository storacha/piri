package app

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/database"
	"github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/fx/identity"
	"github.com/storacha/piri/pkg/fx/proofs"
	"github.com/storacha/piri/pkg/fx/store"
)

func CommonModules(cfg app.AppConfig) fx.Option {
	var modules = []fx.Option{
		// Supply top level config, and it's sub-configs
		// this allows dependencies to be taken on, for example, app.ServerConfig or app.StorageConfig
		// instead of needing to depend on the top level app.AppConfig
		fx.Supply(cfg),
		fx.Supply(cfg.Identity),
		fx.Supply(cfg.Server),
		fx.Supply(cfg.Storage),
		fx.Supply(cfg.UCANService),
		fx.Supply(cfg.PDPService),
		fx.Supply(cfg.Replicator),
		fx.Supply(cfg.PDPService.SigningServiceConfig),

		identity.Module, // Provides principal.Signer
		proofs.Module,   // Provides service for requesting service proofs
		echo.Module,     // Provides Echo server with route registration
		database.Module, // Provides SQLite database for job queues
	}

	if cfg.Storage.DataDir == "" {
		modules = append(modules, store.MemoryStoreModule)
	} else {
		modules = append(modules, store.FileSystemStoreModule)
	}

	return fx.Module("common", modules...)

}
