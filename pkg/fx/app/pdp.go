package app

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/blockchain"
	"github.com/storacha/piri/pkg/fx/database"
	"github.com/storacha/piri/pkg/fx/pdp"
	"github.com/storacha/piri/pkg/fx/scheduler"
	"github.com/storacha/piri/pkg/fx/store"
	"github.com/storacha/piri/pkg/fx/wallet"
)

func PDPServiceModule(cfg app.AppConfig) fx.Option {
	var modules = []fx.Option{
		// Provides a wallet, backed by keystore from storage module
		wallet.Module,
		// Provides lotus and ethereum APIs based on app.AppConfig
		// Provides the PDP Contract interface
		blockchain.Module,
		// Provides various sqlite databases for replication, aggregation, and task engine
		database.Module,
		// Provides chain scheduler, task engine, and various tasks
		scheduler.Module,
		// Provides the PDP Service
		pdp.Module,
	}

	if cfg.Storage.DataDir == "" {
		modules = append(modules, store.MemoryStoreModule)
	} else {
		modules = append(modules, store.FileSystemStoreModule)
	}

	return fx.Module("pdp-service", modules...)
}
