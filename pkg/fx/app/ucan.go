package app

import (
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/fx/blobs"
	"github.com/storacha/piri/pkg/fx/claims"
	"github.com/storacha/piri/pkg/fx/presigner"
	"github.com/storacha/piri/pkg/fx/principalresolver"
	"github.com/storacha/piri/pkg/fx/publisher"
	"github.com/storacha/piri/pkg/fx/replicator"
	"github.com/storacha/piri/pkg/fx/retrieval"
	retrievalucan "github.com/storacha/piri/pkg/fx/retrieval/ucan"
	"github.com/storacha/piri/pkg/fx/root"
	"github.com/storacha/piri/pkg/fx/storage"
	storageucan "github.com/storacha/piri/pkg/fx/storage/ucan"
	"github.com/storacha/piri/pkg/service/egresstracking"
)

var UCANModule = fx.Module("ucan",
	presigner.Module,         // Provides presigner.RequestPresigner
	root.Module,              // Provides root http handler
	blobs.Module,             // Provides blob service and handler
	claims.Module,            // Provides claims service and handler
	publisher.Module,         // Provides publisher service and handler
	egresstracking.Module,    // Provides egress tracking service
	replicator.Module,        // Provides replicator service (works with or without PDP)
	storage.Module,           // Provides storage service wrapper
	retrieval.Module,         // Provides retrieval service wrapper
	principalresolver.Module, // Provides principal resolver for UCAN
	storageucan.Module,       // Provides storage UCAN handler
	retrievalucan.Module,     // Provides retrieval UCAN handler
)
