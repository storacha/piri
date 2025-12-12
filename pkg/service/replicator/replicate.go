package replicator

import (
	"context"

	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/lib/jobqueue"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	replicahandler "github.com/storacha/piri/pkg/service/storage/handlers/replica"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

type Replicator interface {
	Replicate(context.Context, *replicahandler.TransferRequest) error
}

type Service struct {
	queue   *jobqueue.JobQueue[*replicahandler.TransferRequest]
	adapter *adapter
	metrics *replicahandler.Metrics
}

type adapter struct {
	id         principal.Signer
	pdp        pdp.PDP
	blobs      blobs.Blobs
	claims     claims.Claims
	receipts   receiptstore.ReceiptStore
	uploadConn client.Connection
}

type Option func(*Service)

func WithMetrics(metrics *replicahandler.Metrics) Option {
	return func(s *Service) {
		s.metrics = metrics
	}
}

func (a adapter) ID() principal.Signer                { return a.id }
func (a adapter) PDP() pdp.PDP                        { return a.pdp }
func (a adapter) Blobs() blobs.Blobs                  { return a.blobs }
func (a adapter) Claims() claims.Claims               { return a.claims }
func (a adapter) Receipts() receiptstore.ReceiptStore { return a.receipts }
func (a adapter) UploadConnection() client.Connection { return a.uploadConn }

func New(
	id principal.Signer,
	p pdp.PDP,
	b blobs.Blobs,
	c claims.Claims,
	rstore receiptstore.ReceiptStore,
	uploadConn client.Connection,
	queue *jobqueue.JobQueue[*replicahandler.TransferRequest],
	opts ...Option,
) (*Service, error) {
	svc := &Service{
		queue: queue,
		adapter: &adapter{
			id:         id,
			pdp:        p,
			blobs:      b,
			claims:     c,
			receipts:   rstore,
			uploadConn: uploadConn,
		},
	}

	for _, opt := range opts {
		opt(svc)
	}
	if svc.metrics == nil {
		svc.metrics = replicahandler.NewMetrics(nil)
	}

	return svc, nil
}

const TransferTaskName = "transfer-task"

func (r *Service) Replicate(ctx context.Context, task *replicahandler.TransferRequest) error {
	return r.queue.Enqueue(ctx, TransferTaskName, task)
}

func (r *Service) RegisterTransferTask(queue *jobqueue.JobQueue[*replicahandler.TransferRequest]) error {
	return queue.Register(TransferTaskName, func(ctx context.Context, request *replicahandler.TransferRequest) error {
		return replicahandler.Transfer(ctx, r.adapter, request, r.metrics)
	}, jobqueue.WithOnFailure(func(ctx context.Context, msg *replicahandler.TransferRequest, err error) error {
		return replicahandler.SendFailureReceipt(ctx, r.adapter, msg, err)
	}))
}
