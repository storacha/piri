package aggregator

import (
	"database/sql"
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/types"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/piri/pkg/config/app"
	"go.uber.org/fx"

	"github.com/storacha/piri/lib/jobqueue"
	"github.com/storacha/piri/lib/jobqueue/serializer"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
)

const (
	CommpQueueName = "commp"
	CommpTaskName  = "compute"

	AggregatorQueueName = "aggregator"
	AggregatorTaskName  = "aggregate"

	ManagerQueueName = "manager"
	ManagerTaskName  = "aggregates"
)

type QueuesOut struct {
	fx.Out
	CommpQueue      jobqueue.Service[multihash.Multihash]
	AggregatorQueue jobqueue.Service[piece.PieceLink]
	SubmissionQueue jobqueue.Service[[]datamodel.Link]
}

type QueuesParams struct {
	fx.In
	DB     *sql.DB `name:"aggregator_db"`
	Config app.AggregationConfig
}

func NewQueues(params QueuesParams) (*QueuesOut, error) {
	commpQueue, err := jobqueue.New[multihash.Multihash](
		CommpQueueName,
		params.DB,
		&serializer.IPLDCBOR[multihash.Multihash]{
			Typ:  &schema.TypeBytes{},
			Opts: captypes.Converters,
		},
		jobqueue.WithLogger(log.With("queue", CommpQueueName)),
		jobqueue.WithMaxRetries(params.Config.CommpCalculator.Queue.Retries),
		jobqueue.WithMaxWorkers(params.Config.CommpCalculator.Queue.Workers),
		jobqueue.WithMaxTimeout(params.Config.CommpCalculator.Queue.RetryDelay),
	)
	if err != nil {
		return nil, fmt.Errorf("creating commp queue: %w", err)
	}

	// Never aggregate a piece that has already been aggregated, these are the default value, coded here for documentation.
	enableDeDup := true
	blockDQLRetries := true
	// this queue will skip tasks for pieces that have already been or are being processed.
	aggregatorQueue, err := jobqueue.New[piece.PieceLink](
		AggregatorQueueName,
		params.DB,
		&serializer.IPLDCBOR[piece.PieceLink]{
			Typ:  aggregate.PieceLinkType(),
			Opts: types.Converters,
		},
		jobqueue.WithDedupQueue(&jobqueue.DedupQueueConfig{
			DedupeEnabled:     &enableDeDup,
			BlockRepeatsOnDLQ: &blockDQLRetries,
		}),
		jobqueue.WithLogger(log.With("queue", AggregatorQueueName)),
		jobqueue.WithMaxRetries(params.Config.Aggregator.Queue.Retries),
		jobqueue.WithMaxWorkers(params.Config.Aggregator.Queue.Workers),
		jobqueue.WithMaxTimeout(params.Config.Aggregator.Queue.RetryDelay),
	)
	if err != nil {
		return nil, fmt.Errorf("creating piece_link job-queue: %w", err)
	}

	managerQueue, err := jobqueue.New[[]datamodel.Link](
		ManagerQueueName,
		params.DB,
		&serializer.IPLDCBOR[[]datamodel.Link]{
			Typ:  bufferTS.TypeByName("AggregateLinks"),
			Opts: captypes.Converters,
		},
		jobqueue.WithLogger(log.With("queue", ManagerQueueName)),
		jobqueue.WithMaxRetries(params.Config.AggregateManager.Queue.Retries),
		jobqueue.WithMaxWorkers(params.Config.AggregateManager.Queue.Workers),
		jobqueue.WithMaxTimeout(params.Config.AggregateManager.Queue.RetryDelay),
	)
	if err != nil {
		return nil, fmt.Errorf("creating piece_link job-queue: %w", err)
	}

	return &QueuesOut{
		CommpQueue:      commpQueue,
		AggregatorQueue: aggregatorQueue,
		SubmissionQueue: managerQueue,
	}, nil
}
