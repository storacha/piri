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
}

type adapter struct {
	id         principal.Signer
	pdp        pdp.PDP
	blobs      blobs.Blobs
	claims     claims.Claims
	receipts   receiptstore.ReceiptStore
	uploadConn client.Connection
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
) (*Service, error) {
	return &Service{
		queue: queue,
		adapter: &adapter{
			id:         id,
			pdp:        p,
			blobs:      b,
			claims:     c,
			receipts:   rstore,
			uploadConn: uploadConn,
		},
	}, nil
}

const TransferTaskName = "transfer-task"

func (r *Service) Replicate(ctx context.Context, task *replicahandler.TransferRequest) error {
	return r.queue.Enqueue(ctx, TransferTaskName, task)
}

func (r *Service) RegisterTransferTask(queue *jobqueue.JobQueue[*replicahandler.TransferRequest]) error {
	return queue.Register(TransferTaskName, func(ctx context.Context, request *replicahandler.TransferRequest) error {
		return replicahandler.Transfer(ctx, r.adapter, request)
	}, jobqueue.WithOnFailure(func(ctx context.Context, msg *replicahandler.TransferRequest, err error) error {
		return replicahandler.SendFailureReceipt(ctx, r.adapter, msg, err)
	}))
}
