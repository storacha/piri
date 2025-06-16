package replicator

import (
	"context"
	"fmt"
	"io"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/jobqueue"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/invocation/ran"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/message"
	"github.com/storacha/go-ucanto/core/receipt"
	ucanfx "github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/pdp"
	blobhandler "github.com/storacha/piri/pkg/services/blob/ucan"
	"github.com/storacha/piri/pkg/services/types"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("service/replicator")

type Service struct {
	queue *jobqueue.JobQueue[*types.TransferRequest]
}

type adapter struct {
	id         principal.Signer
	pdp        pdp.PDP
	blobs      types.Blobs
	claims     types.Claims
	receipts   receiptstore.ReceiptStore
	uploadConn client.Connection
}

func (a adapter) ID() principal.Signer                { return a.id }
func (a adapter) PDP() pdp.PDP                        { return a.pdp }
func (a adapter) Blobs() types.Blobs                  { return a.blobs }
func (a adapter) Claims() types.Claims                { return a.claims }
func (a adapter) Receipts() receiptstore.ReceiptStore { return a.receipts }
func (a adapter) UploadConnection() client.Connection { return a.uploadConn }

type Params struct {
	fx.In
	ID               principal.Signer
	PDP              pdp.PDP
	Blobs            types.Blobs
	Claims           types.Claims
	ReceiptStore     receiptstore.ReceiptStore
	UploadConnection client.Connection `name:"upload"`
}

func NewService(params Params, lc fx.Lifecycle) (*Service, error) {
	replicationQueue := jobqueue.NewJobQueue[*types.TransferRequest](
		jobqueue.JobHandler(func(ctx context.Context, request *types.TransferRequest) error {
			return Transfer(ctx,
				&adapter{
					id:         params.ID,
					pdp:        params.PDP,
					blobs:      params.Blobs,
					claims:     params.Claims,
					receipts:   params.ReceiptStore,
					uploadConn: params.UploadConnection,
				},
				request)
		}),
		jobqueue.WithErrorHandler(func(err error) {
			log.Errorf("error while handling replication request: %s", err)
		}),
	)

	svc := &Service{queue: replicationQueue}

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("starting replicator service")
			return svc.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			log.Info("stopping replicator service")
			return svc.Stop(ctx)
		},
	})

	return svc, nil
}

func (r *Service) Replicate(ctx context.Context, task *types.TransferRequest) error {
	return r.queue.Queue(ctx, task)
}

func (r *Service) Start(_ context.Context) error {
	r.queue.Startup()
	return nil
}

func (r *Service) Stop(ctx context.Context) error {
	return r.queue.Shutdown(ctx)
}

type TransferService interface {
	// ID is the storage service identity, used to sign UCAN invocations and receipts.
	ID() principal.Signer
	// PDP handles PDP aggregation
	PDP() pdp.PDP
	// Blobs provides access to the blobs service.
	Blobs() types.Blobs
	// Claims provides access to the claims service.
	Claims() types.Claims
	// Receipts provides access to receipts
	Receipts() receiptstore.ReceiptStore
	// UploadConnection provides access to an upload service connection
	UploadConnection() client.Connection
}

func Transfer(ctx context.Context, service TransferService, request *types.TransferRequest) error {
	// pull the data from the source if required
	if request.Sink != nil {
		replicaResp, err := http.Get(request.Source.String())
		if err != nil {
			return fmt.Errorf("http get replication source (%s) failed: %w", request.Source.String(), err)
		}

		// stream the source to the sink
		req, err := http.NewRequest(http.MethodPut, request.Sink.String(), replicaResp.Body)
		if err != nil {
			return fmt.Errorf("failed to create replication sink request: %w", err)
		}
		req.Header = replicaResp.Header
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf(
				"failed http PUT to replicate blob %s from %s to %s failed: %w",
				request.Blob.Digest,
				request.Source.String(),
				request.Sink.String(),
				err,
			)
		}
		// verify status codes
		if res.StatusCode >= 300 || res.StatusCode < 200 {
			topErr := fmt.Errorf(
				"unsuccessful http PUT to replicate blob %s from %s to %s status code %d",
				request.Blob.Digest,
				request.Source.String(),
				request.Sink.String(),
				res.StatusCode,
			)
			resData, err := io.ReadAll(res.Body)
			if err != nil {
				return fmt.Errorf("%s failed to read replication sink response body: %w", topErr, err)
			}
			return fmt.Errorf("%s response body: %s: %w", topErr, resData, err)
		}
	}

	acceptResp, err := blobhandler.Accept(ctx, service, &blobhandler.AcceptRequest{
		Space: request.Space,
		Blob:  request.Blob,
		Put: blob.Promise{
			UcanAwait: blob.Await{
				Selector: ".out.ok",
				Link:     request.Cause.Link(),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to accept replication source blob %s: %w", request.Blob.Digest, err)
	}

	res := replica.TransferOk{
		Site: acceptResp.Claim.Link(),
	}
	forks := []ucanfx.Effect{ucanfx.FromInvocation(acceptResp.Claim)}

	if acceptResp.PDP != nil {
		forks = append(forks, ucanfx.FromInvocation(acceptResp.PDP))
		tmp := acceptResp.PDP.Link()
		res.PDP = &tmp
	}

	ok := result.Ok[replica.TransferOk, ipld.Builder](res)
	var opts []receipt.Option
	if len(forks) > 0 {
		opts = append(opts, receipt.WithFork(forks...))
	}
	rcpt, err := receipt.Issue(service.ID(), ok, ran.FromInvocation(request.Cause), opts...)
	if err != nil {
		return fmt.Errorf("issuing receipt: %w", err)
	}
	if err := service.Receipts().Put(ctx, rcpt); err != nil {
		return fmt.Errorf("failed to put transfer receipt: %w", err)
	}

	msg, err := message.Build([]invocation.Invocation{request.Cause}, []receipt.AnyReceipt{rcpt})
	if err != nil {
		return fmt.Errorf("building message for receipt failed: %w", err)
	}

	uploadServiceRequest, err := service.UploadConnection().Codec().Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message for receipt to http request: %w", err)
	}

	uploadServiceResponse, err := service.UploadConnection().Channel().Request(uploadServiceRequest)
	if err != nil {
		return fmt.Errorf("failed to send request for receipt: %w", err)
	}
	if uploadServiceResponse.Status() >= 300 || uploadServiceResponse.Status() < 200 {
		topErr := fmt.Errorf("unsuccessful http POST to upload service")
		resData, err := io.ReadAll(uploadServiceResponse.Body())
		if err != nil {
			return fmt.Errorf("%s failed to read replication sink response body: %w", topErr, err)
		}
		return fmt.Errorf("%s response body: %s: %w", topErr, resData, err)
	}

	return nil
}
