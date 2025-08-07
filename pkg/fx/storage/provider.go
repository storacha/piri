package storage

import (
	"context"

	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/service/replicator"
	"github.com/storacha/piri/pkg/service/storage"
	"github.com/storacha/piri/pkg/service/storage/ucan"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var Module = fx.Module("storage",
	fx.Provide(
		fx.Annotate(
			NewStorageService,
			fx.As(new(storage.Service)),
			fx.As(new(ucan.BlobAllocateService)),
			fx.As(new(ucan.BlobAcceptService)),
			fx.As(new(ucan.PDPInfoService)),
			fx.As(new(ucan.ReplicaAllocateService)),
		),
	),
)

// StorageServiceParams contains all dependencies for the storage service
type StorageServiceParams struct {
	fx.In

	Config       app.AppConfig
	ID           principal.Signer
	Blobs        blobs.Blobs
	Claims       claims.Claims
	PDP          pdp.PDP
	ReceiptStore receiptstore.ReceiptStore
	Replicator   replicator.Replicator
}

// storageServiceWrapper wraps the storage service to implement the storage.Service interface
type storageServiceWrapper struct {
	id           principal.Signer
	blobs        blobs.Blobs
	claims       claims.Claims
	pdp          pdp.PDP
	receiptStore receiptstore.ReceiptStore
	replicator   replicator.Replicator
	uploadConn   client.Connection
}

// NewStorageService creates a new storage service
func NewStorageService(params StorageServiceParams, lc fx.Lifecycle) (storage.Service, error) {
	svc := &storageServiceWrapper{
		id:           params.ID,
		blobs:        params.Blobs,
		claims:       params.Claims,
		pdp:          params.PDP,
		receiptStore: params.ReceiptStore,
		replicator:   params.Replicator,
		uploadConn:   params.Config.Services.UploadService.Connection,
	}

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// The startup logic is now handled by individual service lifecycle hooks
			return nil
		},
		OnStop: func(ctx context.Context) error {
			// The cleanup logic is now handled by individual service lifecycle hooks
			return nil
		},
	})

	return svc, nil
}

// Implement storage.Service interface
func (s *storageServiceWrapper) ID() principal.Signer {
	return s.id
}

func (s *storageServiceWrapper) PDP() pdp.PDP {
	return s.pdp
}

func (s *storageServiceWrapper) Blobs() blobs.Blobs {
	return s.blobs
}

func (s *storageServiceWrapper) Claims() claims.Claims {
	return s.claims
}

func (s *storageServiceWrapper) Receipts() receiptstore.ReceiptStore {
	return s.receiptStore
}

func (s *storageServiceWrapper) Replicator() replicator.Replicator {
	return s.replicator
}

func (s *storageServiceWrapper) UploadConnection() client.Connection {
	return s.uploadConn
}
