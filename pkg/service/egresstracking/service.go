package egresstracking

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/capabilities/space/egress"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/store/egressbatchstore"
)

// EgressTrackingService stores receipts from `space/content/retrieve` invocations and batches them.
// When batches reaches a certain size, they are sent to the egress tracking service via
// `space/egress/track` invocations.
type EgressTrackingService struct {
	mu                 sync.Mutex
	id                 principal.Signer
	egressTrackerDID   did.DID
	egressTrackerProof delegation.Proof
	egressTrackerConn  client.Connection
	batchEndpoint      *url.URL
	store              egressbatchstore.EgressBatchStore
}

func New(
	id principal.Signer,
	egressTrackerDID did.DID,
	egressTrackerProof delegation.Proof,
	egressTrackerConn client.Connection,
	batchEndpoint *url.URL,
	store egressbatchstore.EgressBatchStore,
) *EgressTrackingService {
	return &EgressTrackingService{
		id:                 id,
		egressTrackerDID:   egressTrackerDID,
		egressTrackerProof: egressTrackerProof,
		egressTrackerConn:  egressTrackerConn,
		batchEndpoint:      batchEndpoint,
		store:              store,
	}
}

func (s *EgressTrackingService) AddReceipt(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	batchRotated, rotatedBatchCID, err := s.store.Append(ctx, rcpt)
	if err != nil {
		return fmt.Errorf("adding receipt to store: %w", err)
	}

	if batchRotated {
		if err := s.trackEgress(ctx, rotatedBatchCID); err != nil {
			return fmt.Errorf("sending egress track invocation: %w", err)
		}
	}

	return nil
}

func (s *EgressTrackingService) trackEgress(ctx context.Context, batchCID cid.Cid) error {
	trackInv, err := egress.Track.Invoke(
		s.id,
		s.egressTrackerDID,
		s.egressTrackerDID.String(),
		egress.TrackCaveats{
			Receipts: cidlink.Link{Cid: batchCID},
			Endpoint: s.batchEndpoint,
		},
		delegation.WithProof(s.egressTrackerProof),
		delegation.WithNoExpiration(),
	)
	if err != nil {
		return fmt.Errorf("creating invocation: %w", err)
	}

	resp, err := client.Execute(ctx, []invocation.Invocation{trackInv}, s.egressTrackerConn)
	if err != nil {
		return fmt.Errorf("executing invocation: %w", err)
	}

	rcptLnk, ok := resp.Get(trackInv.Link())
	if !ok {
		return fmt.Errorf("missing receipt for invocation: %s", trackInv.Link().String())
	}

	blocks, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(resp.Blocks()))
	if err != nil {
		return fmt.Errorf("importing response blocks into blockstore: %w", err)
	}

	rcptReader, err := egress.NewTrackReceiptReader()
	if err != nil {
		return fmt.Errorf("constructing receipt reader: %w", err)
	}

	rcpt, err := rcptReader.Read(rcptLnk, blocks.Iterator())
	if err != nil {
		return fmt.Errorf("reading receipt: %w", err)
	}

	// we're not expecting any meaningful response here so we just check for error
	_, x := result.Unwrap(rcpt.Out())
	var emptyErr egress.TrackError
	if x != emptyErr {
		return fmt.Errorf("invocation failed: %s", x.Message)
	}

	return nil
}
