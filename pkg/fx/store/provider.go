package store

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/store/filesystem"
	"github.com/storacha/piri/pkg/fx/store/memory"
)

func ProvideStores(cfg app.RepoConfig) fx.Option {
	if cfg.DataDir == "" {
		return memory.Module
	} else {
		return filesystem.Module
	}
}
