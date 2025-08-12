package aggregator

import (
	"database/sql"
	"fmt"
	"runtime"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/piece/piece"

	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
)

func NewLinkQueue(db *sql.DB) (*jobqueue.JobQueue[datamodel.Link], error) {
	linkQueue, err := jobqueue.New[datamodel.Link](
		LinkQueueName,
		db,
		&serializer.IPLDCBOR[datamodel.Link]{
			Typ:  &schema.TypeLink{},
			Opts: types.Converters,
		},
		jobqueue.WithLogger(logging.Logger("jobqueue").With("queue", LinkQueueName)),
		jobqueue.WithMaxRetries(50),
		jobqueue.WithMaxWorkers(uint(runtime.NumCPU())),
	)
	if err != nil {
		return nil, fmt.Errorf("creating link job-queue: %w", err)
	}
	return linkQueue, nil
}

func NewPieceQueue(db *sql.DB) (*jobqueue.JobQueue[piece.PieceLink], error) {
	pieceQueue, err := jobqueue.New[piece.PieceLink](
		PieceQueueName,
		db,
		&serializer.IPLDCBOR[piece.PieceLink]{
			Typ:  aggregate.PieceLinkType(),
			Opts: types.Converters,
		},
		jobqueue.WithLogger(logging.Logger("jobqueue").With("queue", PieceQueueName)),
		jobqueue.WithMaxRetries(50),
		jobqueue.WithMaxWorkers(uint(runtime.NumCPU())),
	)
	if err != nil {
		return nil, fmt.Errorf("creating piece_link job-queue: %w", err)
	}
	return pieceQueue, nil
}
