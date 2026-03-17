package store

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/store/filesystem"
	"github.com/storacha/piri/pkg/fx/store/memory"
	"github.com/storacha/piri/pkg/fx/store/s3"
)

// StorageModule returns the appropriate storage module based on configuration.
// If S3 is configured, returns S3Module + LocalOnlyModule.
// Otherwise, returns the full filesystem or memory module.
func StorageModule(cfg app.StorageConfig) fx.Option {
	if cfg.S3 != nil && cfg.S3.Endpoint != "" && cfg.S3.BucketPrefix != "" {
		// Use S3 for most stores, but LocalOnlyModule for stores that must remain
		// on the local filesystem (AggregatorDatastore, PublisherStore, RetrievalJournal, KeyStore)
		return fx.Options(
			s3.Module,
			filesystem.LocalOnlyModule,
		)
	} else if cfg.DataDir == "" {
		return memory.Module
	}
	return filesystem.Module
}
