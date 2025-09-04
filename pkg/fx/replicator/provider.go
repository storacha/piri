package replicator

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/service/replicator"
	replicahandler "github.com/storacha/piri/pkg/service/storage/handlers/replica"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("replicator")

var Module = fx.Module("replicator",
	fx.Provide(
		ProvideReplicationQueue,
		fx.Annotate(
			New,
			fx.As(new(replicator.Replicator)),
		),
	),
	fx.Invoke(
		RegisterReplicationJobs,
	),
)

type QueueParams struct {
	fx.In
	DB *sql.DB `name:"replicator_db"`
}

func ProvideReplicationQueue(lc fx.Lifecycle, params QueueParams) (*jobqueue.JobQueue[*replicahandler.TransferRequest], error) {
	replicationQueue, err := jobqueue.New[*replicahandler.TransferRequest](
		"replication",
		params.DB,
		&serializer.JSON[*replicahandler.TransferRequest]{},
		jobqueue.WithLogger(log.With("queue", "replication")),
		jobqueue.WithMaxRetries(10),
		jobqueue.WithMaxWorkers(uint(runtime.NumCPU())),
	)
	if err != nil {
		return nil, fmt.Errorf("creating replication queue: %w", err)
	}

	queueCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go replicationQueue.Start(queueCtx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel() // Cancel the Start context first
			return replicationQueue.Stop(ctx) // Then wait for graceful shutdown
		},
	})

	return replicationQueue, nil
}

type Params struct {
	fx.In

	Config       app.AppConfig
	ID           principal.Signer
	PDP          pdp.PDP `optional:"true"`
	Blobs        blobs.Blobs
	Claims       claims.Claims
	ReceiptStore receiptstore.ReceiptStore
	Queue        *jobqueue.JobQueue[*replicahandler.TransferRequest]
}

func New(params Params) (*replicator.Service, error) {
	r, err := replicator.New(
		params.ID,
		params.PDP,
		params.Blobs,
		params.Claims,
		params.ReceiptStore,
		params.Config.UCANService.Services.Upload.Connection,
		params.Queue,
	)
	if err != nil {
		return nil, fmt.Errorf("new replicator: %w", err)
	}

	return r, nil
}

func RegisterReplicationJobs(
	queue *jobqueue.JobQueue[*replicahandler.TransferRequest],
	service *replicator.Service,
) error {
	return service.RegisterTransferTask(queue)
}
