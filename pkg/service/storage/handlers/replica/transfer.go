package replica

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/storacha/go-libstoracha/capabilities/access"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	pdp_cap "github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/capabilities/types"
	ucan_cap "github.com/storacha/go-libstoracha/capabilities/ucan"
	"github.com/storacha/go-ucanto/client"
	rclient "github.com/storacha/go-ucanto/client/retrieval"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/receipt/ran"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ucan_http "github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/go-ucanto/validator"
	"go.opentelemetry.io/otel/attribute"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	blobhandler "github.com/storacha/piri/pkg/service/storage/handlers/blob"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("storage/handlers/replica")

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

type TransferSource struct {
	// Identity of the node to transfer from.
	ID ucan.Principal
	// URL the blob may be requested from.
	URL url.URL
}

type transferSourceModel struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type TransferRequest struct {
	// Space is the space to associate with blob.
	Space did.DID
	// Blob is the blob in question.
	Blob types.Blob
	// Source is the location to replicate the blob from.
	Source TransferSource
	// Sink is the location to replicate the blob to.
	Sink *url.URL
	// Cause is the invocation responsible for spawning this replication
	// should be a replica/transfer invocation.
	Cause invocation.Invocation
}

type transferRequestModel struct {
	Space  string              `json:"space"`
	Blob   types.Blob          `json:"blob"`
	Source transferSourceModel `json:"source"`
	Sink   *string             `json:"sink,omitempty"`
	Cause  []byte              `json:"cause"`
}

func (t *TransferRequest) MarshalJSON() ([]byte, error) {
	aux := transferRequestModel{
		Space: t.Space.String(),
		Blob:  t.Blob,
		Source: transferSourceModel{
			ID:  t.Source.ID.DID().String(),
			URL: t.Source.URL.String(),
		},
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
	aux := transferRequestModel{}
	if err := json.Unmarshal(b, &aux); err != nil {
		return fmt.Errorf("unmarshaling TransferRequest: %w", err)
	}

	spaceDID, err := did.Parse(aux.Space)
	if err != nil {
		return fmt.Errorf("parsing space DID: %w", err)
	}
	t.Space = spaceDID

	t.Blob = aux.Blob

	sourceID, err := did.Parse(aux.Source.ID)
	if err != nil {
		return fmt.Errorf("parsing source DID: %w", err)
	}
	t.Source.ID = sourceID
	sourceURL, err := url.Parse(aux.Source.URL)
	if err != nil {
		return fmt.Errorf("parsing source URL: %w", err)
	}
	t.Source.URL = *sourceURL

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

// Transfer handles blob replication with idempotent behavior to support reliable retries.
//
// This function is called by a job queue that retries failed operations up to 10 times.
// To prevent redundant data transfers when retries occur, the function is carefully
// structured to be idempotent:
//
// 1. Always check if the blob already exists BEFORE attempting any transfer
// 2. Only transfer data from source to sink if the blob doesn't exist locally
// 3. If the blob exists (from a previous attempt), skip transfer and just issue receipts
//
// The function handles two distinct scenarios:
// - New blob (request.Sink != nil && !exists): Transfer from source → sink → accept → receipt
// - Existing blob (exists || no sink): Create location assertion → receipt
//
// Both paths end with sending the receipt to the upload service, which confirms
// successful replication to the requesting node.
func Transfer(ctx context.Context, service TransferService, request *TransferRequest, metrics *Metrics) (err error) {
	var (
		rcpt  receipt.AnyReceipt
		forks []fx.Effect
	)

	stopwatch := metrics.startDuration(sourceLabel(&request.Source.URL), sinkLabel(request.Sink))
	defer func() {
		success := true
		if err != nil {
			success = false
		}
		stopwatch.Stop(ctx, attribute.Bool("success", success))
	}()

	// Check if the blob already exists
	blobExists, err := checkBlobExists(ctx, service, request.Blob)
	if err != nil {
		return fmt.Errorf("checking if blob has been received before transfer: %w", err)
	}

	if request.Sink != nil && !blobExists {
		// Need to transfer the blob from source to sink
		acceptResp, err := transferBlobFromSource(ctx, service, request)
		if err != nil {
			return fmt.Errorf("failed to accept replication source blob %s: %w", request.Blob.Digest, err)
		}

		forks = []fx.Effect{fx.FromInvocation(acceptResp.Claim)}
		var pdpLink *ipld.Link
		if acceptResp.PDP != nil {
			forks = append(forks, fx.FromInvocation(acceptResp.PDP))
			tmp := acceptResp.PDP.Link()
			pdpLink = &tmp
		}

		rcpt, err = issueTransferReceipt(ctx, service, request, acceptResp.Claim.Link(), pdpLink, forks)
		if err != nil {
			return err
		}
	} else {
		// Blob already exists (skip transfer for idempotency) or no sink specified - create location assertion
		claim, pdpAcceptInv, err := createLocationAssertion(ctx, service, request)
		if err != nil {
			return err
		}

		forks = []fx.Effect{fx.FromInvocation(claim)}
		var pdpLink *ipld.Link
		if pdpAcceptInv != nil {
			forks = append(forks, fx.FromInvocation(pdpAcceptInv))
			tmp := pdpAcceptInv.Link()
			pdpLink = &tmp
		}

		rcpt, err = issueTransferReceipt(ctx, service, request, claim.Link(), pdpLink, forks)
		if err != nil {
			return err
		}
	}

	// Build and send message to upload service
	return sendMessageToUploadService(ctx, service, rcpt)
}

func sinkLabel(sink *url.URL) string {
	if sink == nil {
		return "none"
	}
	return sink.String()
}

func sourceLabel(source *url.URL) string {
	if source == nil {
		return "none"
	}
	return source.String()
}

// checkBlobExists checks if the blob already exists in either PDP or Blobs store
func checkBlobExists(ctx context.Context, service TransferService, blob types.Blob) (bool, error) {
	var err error
	if service.PDP() != nil {
		has, err := service.PDP().API().Has(ctx, blob.Digest)
		if err != nil {
			return false, fmt.Errorf("resolving Piece: %w", err)
		}
		return has, nil
	}
	_, err = service.Blobs().Store().Get(ctx, blob.Digest)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("checking if blob exists: %w", err)
}

// transferBlobFromSource fetches blob from source and PUTs it to sink
func transferBlobFromSource(ctx context.Context, service TransferService, request *TransferRequest) (*blobhandler.AcceptResponse, error) {
	allocInv, err := extractReplicaAllocateInvocation(request.Cause)
	if err != nil {
		return nil, fmt.Errorf("extracting %s invocation: %w", replica.AllocateAbility, err)
	}

	dlg, err := requestBlobRetrieveDelegation(ctx, request.Source.URL, service.ID(), request.Source.ID, allocInv)
	if err != nil {
		return nil, fmt.Errorf("requesting %s delegation: %w", blob.RetrieveAbility, err)
	}

	// perform authorized retrieval from source using the delegation
	inv, err := blob.Retrieve.Invoke(
		service.ID(),
		request.Source.ID,
		request.Source.ID.DID().String(),
		blob.RetrieveCaveats{Blob: blob.Blob{Digest: request.Blob.Digest}},
		delegation.WithProof(delegation.FromDelegation(dlg)),
	)
	if err != nil {
		return nil, fmt.Errorf("creating %s invocation: %w", blob.RetrieveAbility, err)
	}

	conn, err := rclient.NewConnection(request.Source.ID, &request.Source.URL)
	if err != nil {
		return nil, fmt.Errorf("creating connection to %s: %w", request.Source.ID.DID(), err)
	}

	replicaExecResp, replicaResp, err := rclient.Execute(ctx, inv, conn)
	if err != nil {
		return nil, fmt.Errorf("executing %s invocation: %w", blob.RetrieveAbility, err)
	}
	defer replicaResp.Body().Close()

	rcptLink, ok := replicaExecResp.Get(inv.Link())
	if !ok {
		return nil, fmt.Errorf("missing %s receipt: %s", blob.RetrieveAbility, inv.Link())
	}

	rcptReader, err := blob.NewRetrieveReceiptReader()
	if err != nil {
		return nil, err
	}

	rcpt, err := rcptReader.Read(rcptLink, replicaExecResp.Blocks())
	if err != nil {
		return nil, fmt.Errorf("reading %s receipt: %w", blob.RetrieveAbility, err)
	}

	_, x := result.Unwrap(rcpt.Out())
	if !errors.Is(x, blob.RetrieveError{}) {
		return nil, fmt.Errorf("replication source (%s) returned failure in receipt: %w", request.Source.URL.String(), x)
	}

	// Verify status from source
	if replicaResp.Status() >= 300 || replicaResp.Status() < 200 {
		return nil, fmt.Errorf("replication source (%s) returned unexpected status: %d", request.Source.URL.String(), replicaResp.Status())
	}

	// Stream source to sink
	req, err := http.NewRequest(http.MethodPut, request.Sink.String(), replicaResp.Body())
	if err != nil {
		return nil, fmt.Errorf("failed to create replication sink request: %w", err)
	}
	req.Header = replicaResp.Headers()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"failed http PUT to replicate blob %s from %s to %s failed: %w",
			request.Blob.Digest,
			request.Source.URL.String(),
			request.Sink.String(),
			err,
		)
	}
	defer res.Body.Close()

	// Verify status
	if res.StatusCode >= 300 || res.StatusCode < 200 {
		topErr := fmt.Errorf(
			"unsuccessful http PUT to replicate blob %s from %s to %s status code %d",
			request.Blob.Digest,
			request.Source.URL.String(),
			request.Sink.String(),
			res.StatusCode,
		)
		resData, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("%s failed to read replication sink response body: %w", topErr, err)
		}
		return nil, fmt.Errorf("%s response body: %s", topErr, resData)
	}

	// Accept the blob
	return blobhandler.Accept(ctx, service, &blobhandler.AcceptRequest{
		Space: request.Space,
		Blob:  request.Blob,
		Put: blob.Promise{
			UcanAwait: blob.Await{
				Selector: ".out.ok",
				Link:     request.Cause.Link(),
			},
		},
		Cause: request.Cause.Link(),
	})
}

// extractReplicaAllocateInvocation extracts the `blob/replica/allocate`
// invocation which is expected to be attached to the `blob/transfer` invocation
func extractReplicaAllocateInvocation(trnsfInv invocation.Invocation) (invocation.Invocation, error) {
	if len(trnsfInv.Capabilities()) != 1 {
		return nil, fmt.Errorf("invalid %s invocation", replica.TransferAbility)
	}
	var err error
	match, err := replica.Transfer.Match(validator.NewSource(trnsfInv.Capabilities()[0], trnsfInv))
	if err != nil {
		return nil, fmt.Errorf("matching %s invocation: %w", replica.TransferAbility, err)
	}
	blocks, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(trnsfInv.Blocks()))
	if err != nil {
		return nil, fmt.Errorf("reading %s invocation blocks: %w", replica.TransferAbility, err)
	}
	return invocation.NewInvocationView(match.Value().Nb().Cause, blocks)
}

// requestBlobRetrieveDelegation obtains a delegation for `blob/retrieve` from a
// node by invoking `access/grant`, using the `blob/replica/allocate` invocation
// as evidence that the delegation should be granted.
func requestBlobRetrieveDelegation(
	ctx context.Context,
	endpoint url.URL,
	issuer ucan.Signer,
	audience ucan.Principal,
	cause invocation.Invocation, // the `blob/replica/allocate` invocation
) (delegation.Delegation, error) {
	inv, err := access.Grant.Invoke(
		issuer,
		audience,
		issuer.DID().String(),
		access.GrantCaveats{
			Att:   []access.CapabilityRequest{{Can: blob.Retrieve.Can()}},
			Cause: cause.Link(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("creating %s invocation: %w", access.GrantAbility, err)
	}
	for b, err := range cause.Export() {
		if err != nil {
			return nil, fmt.Errorf("exporting %s blocks: %w", replica.AllocateAbility, err)
		}
		if err = inv.Attach(b); err != nil {
			return nil, fmt.Errorf("attaching %s blocks: %w", replica.AllocateAbility, err)
		}
	}

	ch := ucan_http.NewChannel(&endpoint)
	conn, err := client.NewConnection(audience, ch)
	if err != nil {
		return nil, fmt.Errorf("creating connection to %s: %w", audience.DID(), err)
	}

	resp, err := client.Execute(ctx, []invocation.Invocation{inv}, conn)
	if err != nil {
		return nil, fmt.Errorf("executing %s invocation: %w", access.GrantAbility, err)
	}

	rcptLink, ok := resp.Get(inv.Link())
	if !ok {
		return nil, fmt.Errorf("missing %s receipt: %s", access.GrantAbility, inv.Link())
	}

	rcptReader, err := access.NewGrantReceiptReader()
	if err != nil {
		return nil, err
	}

	rcpt, err := rcptReader.Read(rcptLink, resp.Blocks())
	if err != nil {
		return nil, fmt.Errorf("reading %s receipt: %w", access.GrantAbility, err)
	}

	return result.MatchResultR2(
		rcpt.Out(),
		func(o access.GrantOk) (delegation.Delegation, error) {
			dlgBytes := o.Delegations.Values[o.Delegations.Keys[0]]
			return delegation.Extract(dlgBytes)
		},
		func(x access.GrantError) (delegation.Delegation, error) {
			return nil, x
		},
	)
}

// createLocationAssertion creates a location assertion for an existing blob
func createLocationAssertion(ctx context.Context, service TransferService, request *TransferRequest) (invocation.Invocation, invocation.Invocation, error) {
	var (
		loc          url.URL
		pdpAcceptInv invocation.Invocation
	)

	if service.PDP() == nil {
		_, err := service.Blobs().Store().Get(ctx, request.Blob.Digest)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, nil, fmt.Errorf("blob not found: %w", err)
			}
			return nil, nil, fmt.Errorf("getting blob: %w", err)
		}

		loc, err = service.Blobs().Access().GetDownloadURL(request.Blob.Digest)
		if err != nil {
			return nil, nil, fmt.Errorf("creating retrieval URL for blob: %w", err)
		}
	} else {
		// Locate the piece from the PDP service
		has, err := service.PDP().API().Has(ctx, request.Blob.Digest)
		if err != nil {
			return nil, nil, fmt.Errorf("finding piece for blob: %w", err)
		}
		if !has {
			return nil, nil, fmt.Errorf("piece not found")
		}

		blobCID := cid.NewCidV1(cid.Raw, request.Blob.Digest)
		loc, err = service.PDP().API().ReadPieceURL(blobCID)
		if err != nil {
			return nil, nil, fmt.Errorf("creating retrieval URL for blob: %w", err)
		}
		// Generate the invocation for piece acceptance
		pieceAccept, err := pdp_cap.Accept.Invoke(
			service.ID(),
			service.ID(),
			service.ID().DID().String(),
			pdp_cap.AcceptCaveats{
				Blob: blobCID.Hash(),
			}, delegation.WithNoExpiration())

		if err != nil {
			return nil, nil, fmt.Errorf("creating piece accept invocation: %w", err)
		}
		pdpAcceptInv = pieceAccept
	}

	claim, err := assert.Location.Delegate(
		service.ID(),
		request.Space,
		service.ID().DID().String(),
		assert.LocationCaveats{
			Space:    request.Space,
			Content:  types.FromHash(request.Blob.Digest),
			Location: []url.URL{loc},
		},
		delegation.WithNoExpiration(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating location commitment: %w", err)
	}

	return claim, pdpAcceptInv, nil
}

// issueTransferReceipt creates and stores a transfer receipt
func issueTransferReceipt(ctx context.Context, service TransferService, request *TransferRequest, siteLink ipld.Link, pdpLink *ipld.Link, forks []fx.Effect) (receipt.AnyReceipt, error) {
	transferReceipt := replica.TransferOk{
		Site: siteLink,
		PDP:  pdpLink,
	}

	ok := result.Ok[replica.TransferOk, ipld.Builder](transferReceipt)
	var opts []receipt.Option
	if len(forks) > 0 {
		opts = append(opts, receipt.WithFork(forks...))
	}

	rcpt, err := receipt.Issue(service.ID(), ok, ran.FromInvocation(request.Cause), opts...)
	if err != nil {
		return nil, fmt.Errorf("issuing receipt: %w", err)
	}

	if err := service.Receipts().Put(ctx, rcpt); err != nil {
		return nil, fmt.Errorf("failed to put transfer receipt: %w", err)
	}

	return rcpt, nil
}

// linksFact is a [ucan.FactBuilder] for IPLD links.
type linksFact []ipld.Link

func (f linksFact) ToIPLD() (map[string]ipld.Node, error) {
	m := map[string]ipld.Node{}
	for _, l := range f {
		nb := basicnode.Prototype.Link.NewBuilder()
		if err := nb.AssignLink(l); err != nil {
			return nil, err
		}
		m[l.String()] = nb.Build()
	}
	return m, nil
}

// sendMessageToUploadService sends the message containing invocations and receipts to the upload service
func sendMessageToUploadService(ctx context.Context, service TransferService, rcpt receipt.AnyReceipt) error {
	var rcptBlocks []ipld.Block
	var rcptBlockLinks linksFact
	for b, err := range rcpt.Blocks() {
		if err != nil {
			return fmt.Errorf("iterating receipt blocks: %w", err)
		}
		rcptBlocks = append(rcptBlocks, b)
		rcptBlockLinks = append(rcptBlockLinks, b.Link())
	}

	concludeInv, err := ucan_cap.Conclude.Invoke(
		service.ID(),
		service.UploadConnection().ID().DID(),
		service.ID().DID().String(),
		ucan_cap.ConcludeCaveats{
			Receipt: rcpt.Root().Link(),
		},
		// ensure all receipt blocks remain included with this invocation
		delegation.WithFacts([]ucan.FactBuilder{rcptBlockLinks}),
	)
	if err != nil {
		return fmt.Errorf("generating conclude invocation: %w", err)
	}

	// attach the receipt blocks to the conclude invocation
	for _, b := range rcptBlocks {
		if err := concludeInv.Attach(b); err != nil {
			return fmt.Errorf("attaching receipt block: %w", err)
		}
	}

	resp, err := client.Execute(ctx, []invocation.Invocation{concludeInv}, service.UploadConnection())
	if err != nil {
		return fmt.Errorf("executing conclude invocation: %w", err)
	}

	concludeRcptLink, ok := resp.Get(concludeInv.Link())
	if !ok {
		return fmt.Errorf("missing receipt for invocation: %s", concludeInv.Link().String())
	}

	blocks, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(resp.Blocks()))
	if err != nil {
		return fmt.Errorf("constructing blockstore: %w", err)
	}

	concludeRcpt, err := receipt.NewAnyReceipt(concludeRcptLink, blocks)
	if err != nil {
		return fmt.Errorf("constructing receipt: %w", err)
	}

	// we're not expecting any meaningful response here so we just check for error
	_, x := result.Unwrap(concludeRcpt.Out())
	if x != nil {
		log.Errorf("conclude invocation failure: %s", printer.Sprint(x))
		return errors.New("conclude invocation failed")
	}

	return nil
}

// SendFailureReceipt sends a failure receipt to the upload service when Transfer fails after all retries
func SendFailureReceipt(ctx context.Context, service TransferService, request *TransferRequest, transferErr error) error {
	failure := replica.NewTransferError(fmt.Sprintf("failed to transfer after all retries: %s", transferErr.Error()))

	errResult := result.Error[replica.TransferOk, replica.TransferError](failure)

	rcpt, err := receipt.Issue(service.ID(), errResult, ran.FromInvocation(request.Cause))
	if err != nil {
		return fmt.Errorf("issuing failure receipt: %w", err)
	}

	if err := service.Receipts().Put(ctx, rcpt); err != nil {
		return fmt.Errorf("failed to store failure receipt: %w", err)
	}

	if err := sendMessageToUploadService(ctx, service, rcpt); err != nil {
		return fmt.Errorf("sending failure receipt: %w", err)
	}

	return nil
}
