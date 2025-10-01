package egresstracking

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	leveldb "github.com/ipfs/go-ds-leveldb"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/principal"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/client/receipts"
	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"github.com/storacha/piri/pkg/store/consolidationstore"
	"github.com/storacha/piri/pkg/store/retrievaljournal"
)

var log = logging.Logger("egresstracking")

var Module = fx.Module("egresstracking",
	fx.Provide(
		ProvideEgressTrackingQueue,
		ProvideConsolidationStore,
		ProvideReceiptsClient,
		NewService,
		fx.Annotate(
			NewServer,
			fx.As(new(echofx.RouteRegistrar)),
			fx.ResultTags(`group:"route_registrar"`),
		),
	),
)

type QueueParams struct {
	fx.In

	DB *sql.DB `name:"egress_tracking_db"`
}

func ProvideEgressTrackingQueue(lc fx.Lifecycle, params QueueParams) (EgressTrackingQueue, error) {
	// non-configurable defaults
	maxRetries := uint(10)
	maxWorkers := uint(runtime.NumCPU())
	maxTimeout := 5 * time.Second

	queue, err := jobqueue.New(
		"egress-tracking",
		params.DB,
		&serializer.JSON[cid.Cid]{},
		jobqueue.WithLogger(log.With("queue", "egress-tracking")),
		jobqueue.WithMaxRetries(maxRetries),
		jobqueue.WithMaxWorkers(maxWorkers),
		jobqueue.WithMaxTimeout(maxTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("creating egress-tracking queue: %w", err)
	}

	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return queue.Start(queueCtx)
		},
		OnStop: func(ctx context.Context) error {
			cancel()               // Cancel the Start context first
			return queue.Stop(ctx) // Then wait for graceful shutdown
		},
	})

	return NewEgressTrackingQueue(queue), nil
}

func ProvideConsolidationStore(lc fx.Lifecycle, cfg app.AppConfig) (consolidationstore.Store, error) {
	baseDir := cfg.Storage.EgressTracking.Dir

	var ds datastore.Datastore
	var err error

	if baseDir == "" {
		// Use memory-based store
		log.Info("using memory-based consolidation store")
		ds = datastore.NewMapDatastore()
	} else {
		// Use leveldb
		dsPath := filepath.Join(baseDir, "consolidation")
		ds, err = leveldb.NewDatastore(dsPath, &leveldb.Options{
			Compression: ldbopts.NoCompression,
		})
		if err != nil {
			return nil, fmt.Errorf("creating leveldb datastore: %w", err)
		}

		// Add lifecycle hook to close leveldb on shutdown
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				if err := ds.Close(); err != nil {
					log.Errorf("error closing consolidation datastore: %v", err)
					return err
				}
				return nil
			},
		})
	}

	return consolidationstore.New(ds), nil
}

func ProvideReceiptsClient(lc fx.Lifecycle, cfg app.AppConfig) *receipts.Client {
	receiptsEndpoint := cfg.UCANService.Services.EgressTracker.ReceiptsEndpoint
	return receipts.NewClient(receiptsEndpoint)
}

func NewService(
	lc fx.Lifecycle,
	id principal.Signer,
	store retrievaljournal.Journal,
	consolidationStore consolidationstore.Store,
	queue EgressTrackingQueue,
	rcptsClient *receipts.Client,
	cfg app.AppConfig,
) (*EgressTrackingService, error) {
	batchEndpoint := cfg.Server.PublicURL.JoinPath(ReceiptsPath + "/{cid}")
	egressTrackerConn := cfg.UCANService.Services.EgressTracker.Connection
	egressTrackerProofs := cfg.UCANService.Services.EgressTracker.Proofs
	receiptsEndpoint := cfg.UCANService.Services.EgressTracker.ReceiptsEndpoint
	cleanupCheckInterval := cfg.UCANService.Services.EgressTracker.CleanupCheckInterval

	if egressTrackerConn == nil {
		log.Warn("no egress tracking service connection provided, egress tracking is disabled")
		return nil, nil
	}

	// Disable cleanup if receipts endpoint is not configured or empty
	if receiptsEndpoint == nil || receiptsEndpoint.String() == "" {
		log.Warn("no egress tracker receipts endpoint configured, cleanup task will be disabled")
		cleanupCheckInterval = 0 // Disable cleanup
	}

	svc, err := New(
		id,
		egressTrackerConn,
		egressTrackerProofs,
		batchEndpoint,
		store,
		receiptsEndpoint,
		consolidationStore,
		queue,
		rcptsClient,
		cleanupCheckInterval,
	)
	if err != nil {
		return nil, err
	}

	// Add lifecycle hooks for cleanup task
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return svc.StartCleanupTask(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return svc.StopCleanupTask(ctx)
		},
	})

	return svc, nil
}
