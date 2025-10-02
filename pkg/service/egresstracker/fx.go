package egresstracker

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	echofx "github.com/storacha/piri/pkg/fx/echo"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"github.com/storacha/piri/pkg/store/retrievaljournal"
)

var log = logging.Logger("egresstracking")

var Module = fx.Module("egresstracking",
	fx.Provide(
		ProvideEgressTrackerQueue,
		NewEgressTrackerService,
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

func ProvideEgressTrackerQueue(lc fx.Lifecycle, params QueueParams) (EgressTrackerQueue, error) {
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

	return NewEgressTrackerQueue(queue), nil
}

func NewEgressTrackerService(
	id principal.Signer,
	store retrievaljournal.Journal,
	queue EgressTrackerQueue,
	cfg app.AppConfig,
) (*Service, error) {
	batchEndpoint := cfg.Server.PublicURL.JoinPath(ReceiptsPath + "/{cid}")
	egressTrackerConn := cfg.UCANService.Services.EgressTracker.Connection
	egressTrackerProofs := cfg.UCANService.Services.EgressTracker.Proofs

	if egressTrackerConn == nil {
		log.Warn("no egress tracking service connection provided, egress tracking is disabled")
		return nil, nil
	}

	return New(
		id,
		egressTrackerConn,
		egressTrackerProofs,
		batchEndpoint,
		store,
		queue,
	)
}
