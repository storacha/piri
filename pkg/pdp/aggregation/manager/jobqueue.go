package manager

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/piri/lib/jobqueue"
	"github.com/storacha/piri/lib/jobqueue/serializer"
	"github.com/storacha/piri/pkg/pdp/aggregation/types"
	pdptypes "github.com/storacha/piri/pkg/pdp/types"
	"go.uber.org/fx"
)

const (
	QueueName   = "manager"
	HandlerName = "add_roots"
)

// NewAddRootsTaskHandler creates a TaskHandler that submits aggregate roots to the PDP Service
func NewAddRootsTaskHandler(
	api pdptypes.ProofSetAPI,
	proofSet pdptypes.ProofSetIDProvider,
	store types.Store,
	accepter *PieceAccepter,
) jobqueue.TaskHandler[[]datamodel.Link] {
	return &AddRootsTaskHandler{
		api:           api,
		proofSet:      proofSet,
		store:         store,
		pieceAccepter: accepter,
	}
}

type AddRootsTaskHandler struct {
	api           pdptypes.ProofSetAPI
	proofSet      pdptypes.ProofSetIDProvider
	store         types.Store
	pieceAccepter *PieceAccepter
}

func (a *AddRootsTaskHandler) Name() string {
	return HandlerName
}

func (a *AddRootsTaskHandler) Handle(ctx context.Context, links []datamodel.Link) error {
	if err := a.pieceAccepter.AcceptPieces(ctx, links); err != nil {
		return fmt.Errorf("failed to accept pieces: %w", err)
	}
	proofSetID, err := a.proofSet.ProofSetID(ctx)
	if err != nil {
		return fmt.Errorf("getting proof set ID from proof set provider: %w", err)
	}

	// build the set of roots we will add
	roots := make([]pdptypes.RootAdd, len(links))
	for i, aggregateLink := range links {
		// fetch each aggregate to submit
		a, err := a.store.Get(ctx, aggregateLink)
		if err != nil {
			return fmt.Errorf("reading aggregates: %w", err)
		}
		// record its root
		rootCID, err := cid.Decode(a.Root.Link().String())
		if err != nil {
			return fmt.Errorf("failed to decode aggregate root CID: %w", err)
		}
		// subroots
		subRoots := make([]cid.Cid, len(a.Pieces))
		for j, p := range a.Pieces {
			pcid, err := cid.Decode(p.Link.Link().String())
			if err != nil {
				return fmt.Errorf("failed to decode piece CID: %w", err)
			}
			subRoots[j] = pcid
		}
		roots[i] = pdptypes.RootAdd{
			Root:     rootCID,
			SubRoots: subRoots,
		}
		log.Infow("root aggregate added", "root", aggregateLink.String())
	}

	txHash, err := a.api.AddRoots(ctx, proofSetID, roots)
	if err != nil {
		return fmt.Errorf("adding roots: %w", err)
	}
	log.Infow("added roots", "count", len(roots), "tx", txHash)
	return nil
}

type QueueParams struct {
	fx.In
	DB *sql.DB `name:"aggregator_db"`
}

func NewQueue(params QueueParams) (jobqueue.Service[[]datamodel.Link], error) {
	managerQueue, err := jobqueue.New[[]datamodel.Link](
		QueueName,
		params.DB,
		&serializer.IPLDCBOR[[]datamodel.Link]{
			Typ:  bufferTS.TypeByName("AggregateLinks"),
			Opts: captypes.Converters,
		},
		jobqueue.WithLogger(log.With("queue", QueueName)),
		jobqueue.WithMaxRetries(50),
		// NB: must remain one to keep submissions serial to AddRoots
		jobqueue.WithMaxWorkers(uint(1)),
		// wait for twice a filecoin epoch to submit
		jobqueue.WithMaxTimeout(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("creating piece_link job-queue: %w", err)
	}
	// NB: queue lifecycle is handled by manager since it must register with queue before starting it
	return managerQueue, nil
}
