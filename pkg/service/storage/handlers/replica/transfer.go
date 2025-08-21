package replica

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/invocation/ran"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/message"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	blobhandler "github.com/storacha/piri/pkg/service/storage/handlers/blob"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("handlers/ucan/replication")

type TransferService interface {
	// ID is the storage service identity, used to sign UCAN invocations and receipts.
	ID() principal.Signer
	// PDP handles PDP aggregation
	PDP() pdp.PDP
	// Blobs provides access to the blobs service.
	Blobs() blobs.Blobs
	// Claims provides access to the claims service.
	Claims() claims.Claims
	// Receipts provides access to receipts
	Receipts() receiptstore.ReceiptStore
	// UploadConnection provides access to an upload service connection
	UploadConnection() client.Connection
}

type TransferRequest struct {
	// Space is the space to associate with blob.
	Space did.DID
	// Blob is the blob in question.
	Blob types.Blob
	// Source is the location to replicate the blob from.
	Source url.URL
	// Sink is the location to replicate the blob to.
	Sink *url.URL
	// Cause is the invocation responsible for spawning this replication
	// should be a replica/transfer invocation.
	Cause invocation.Invocation
}

func (t *TransferRequest) MarshalJSON() ([]byte, error) {
	aux := struct {
		Space  string     `json:"space"`
		Blob   types.Blob `json:"blob"`
		Source string     `json:"source"`
		Sink   *string    `json:"sink,omitempty"`
		Cause  []byte     `json:"cause"`
	}{
		Space:  t.Space.String(),
		Blob:   t.Blob,
		Source: t.Source.String(),
	}

	if t.Sink != nil {
		sinkStr := t.Sink.String()
		aux.Sink = &sinkStr
	}

	causeBytes, err := io.ReadAll(t.Cause.Archive())
	if err != nil {
		return nil, fmt.Errorf("marshaling cause: %w", err)
	}
	aux.Cause = causeBytes

	return json.Marshal(aux)
}

func (t *TransferRequest) UnmarshalJSON(b []byte) error {
	aux := struct {
		Space  string     `json:"space"`
		Blob   types.Blob `json:"blob"`
		Source string     `json:"source"`
		Sink   *string    `json:"sink,omitempty"`
		Cause  []byte     `json:"cause"`
	}{}

	if err := json.Unmarshal(b, &aux); err != nil {
		return fmt.Errorf("unmarshaling TransferRequest: %w", err)
	}

	spaceDID, err := did.Parse(aux.Space)
	if err != nil {
		return fmt.Errorf("parsing space DID: %w", err)
	}
	t.Space = spaceDID

	t.Blob = aux.Blob

	sourceURL, err := url.Parse(aux.Source)
	if err != nil {
		return fmt.Errorf("parsing source URL: %w", err)
	}
	t.Source = *sourceURL

	if aux.Sink != nil {
		sinkURL, err := url.Parse(*aux.Sink)
		if err != nil {
			return fmt.Errorf("parsing sink URL: %w", err)
		}
		t.Sink = sinkURL
	}

	inv, err := delegation.Extract(aux.Cause)
	if err != nil {
		return fmt.Errorf("unmarshaling cause: %w", err)
	}
	t.Cause = inv

	return nil
}

func Transfer(ctx context.Context, service TransferService, request *TransferRequest) error {
	l := log.With("space", request.Space, "blob", request.Blob, "source", request.Source, "sink", request.Sink, "cause", request.Cause.Root().Link().String())
	// pull the data from the source if required
	if request.Sink != nil {
		l.Info("pulling data from replication source")
		replicaResp, err := http.Get(request.Source.String())
		if err != nil {
			l.Errorw("failed to get data from replication source", "error", err)
			return fmt.Errorf("http get replication source (%s) failed: %w", request.Source.String(), err)
		}

		// stream the source to the sink
		l.Info("putting data to replication sink")
		req, err := http.NewRequest(http.MethodPut, request.Sink.String(), replicaResp.Body)
		if err != nil {
			l.Errorw("failed to create request for data from replication sink", "error", err)
			return fmt.Errorf("failed to create replication sink request: %w", err)
		}
		req.Header = replicaResp.Header
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			l.Errorw("failed to put data to replication sink", "error", err)
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
			l.Errorw("received unexpected status from replication sink put", "status", res.StatusCode, "error", topErr)
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
		l.Errorw("failed to blob accept replicated data", "error", err)
		return fmt.Errorf("failed to accept replication source blob %s: %w", request.Blob.Digest, err)
	}

	res := replica.TransferOk{
		Site: acceptResp.Claim.Link(),
	}
	forks := []fx.Effect{fx.FromInvocation(acceptResp.Claim)}

	if acceptResp.PDP != nil {
		forks = append(forks, fx.FromInvocation(acceptResp.PDP))
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
		l.Errorw("failed to put receipt", "error", err)
		return fmt.Errorf("failed to put transfer receipt: %w", err)
	}

	msg, err := message.Build([]invocation.Invocation{request.Cause}, []receipt.AnyReceipt{rcpt})
	if err != nil {
		l.Errorw("failed to build message", "error", err)
		return fmt.Errorf("building message for receipt failed: %w", err)
	}

	uploadServiceRequest, err := service.UploadConnection().Codec().Encode(msg)
	if err != nil {
		l.Errorw("failed to encode upload connection", "error", err)
		return fmt.Errorf("failed to encode message for receipt to http request: %w", err)
	}

	uploadServiceResponse, err := service.UploadConnection().Channel().Request(ctx, uploadServiceRequest)
	if err != nil {
		l.Errorw("failed to send upload connection", "error", err)
		return fmt.Errorf("failed to send request for receipt: %w", err)
	}
	if uploadServiceResponse.Status() >= 300 || uploadServiceResponse.Status() < 200 {
		topErr := fmt.Errorf("unsuccessful http POST to upload service")
		l.Errorw("upload service returned unexpected response", "error", err, "status", uploadServiceResponse.Status())
		resData, err := io.ReadAll(uploadServiceResponse.Body())
		if err != nil {
			return fmt.Errorf("%s failed to read replication sink response body: %w", topErr, err)
		}
		return fmt.Errorf("%s response body: %s: %w", topErr, resData, err)
	}

	return nil
}
