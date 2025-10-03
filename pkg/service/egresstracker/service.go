package egresstracker

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/capabilities/space/egress"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/store/retrievaljournal"
)

// Service stores receipts from `space/content/retrieve` invocations, batches them and sends
// them to an egress tracking service via `space/egress/track` invocations.
type Service struct {
	id                  principal.Signer
	egressTrackerDID    did.DID
	egressTrackerProofs delegation.Proofs
	egressTrackerConn   client.Connection
	batchEndpoint       *url.URL
	store               retrievaljournal.Journal
	queue               EgressTrackerQueue
}

func New(
	id principal.Signer,
	egressTrackerConn client.Connection,
	egressTrackerProofs delegation.Proofs,
	batchEndpoint *url.URL,
	store retrievaljournal.Journal,
	queue EgressTrackerQueue,
) (*Service, error) {
	svc := &Service{
		id:                  id,
		egressTrackerDID:    egressTrackerConn.ID().DID(),
		egressTrackerProofs: egressTrackerProofs,
		egressTrackerConn:   egressTrackerConn,
		batchEndpoint:       batchEndpoint,
		store:               store,
		queue:               queue,
	}

	if err := queue.Register(svc.egressTrack); err != nil {
		return nil, fmt.Errorf("registering egress track task: %w", err)
	}

	return svc, nil
}

func (s *Service) AddReceipt(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) error {
	batchRotated, rotatedBatchCID, err := s.store.Append(ctx, rcpt)
	if err != nil {
		return fmt.Errorf("adding receipt to store: %w", err)
	}

	if batchRotated {
		if err := s.enqueueEgressTrackTask(ctx, rotatedBatchCID); err != nil {
			return fmt.Errorf("enqueuing egress track task: %w", err)
		}
	}

	return nil
}

func (s *Service) enqueueEgressTrackTask(ctx context.Context, batchCID cid.Cid) error {
	return s.queue.Enqueue(ctx, batchCID)
}

func (s *Service) egressTrack(ctx context.Context, batchCID cid.Cid) error {
	trackInv, err := egress.Track.Invoke(
		s.id,
		s.egressTrackerDID,
		s.egressTrackerDID.String(),
		egress.TrackCaveats{
			Receipts: cidlink.Link{Cid: batchCID},
			Endpoint: s.batchEndpoint,
		},
		delegation.WithProof(s.egressTrackerProofs...),
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

	rcptReader, err := receipt.NewReceiptReaderFromTypes[egress.TrackOk, fdm.FailureModel](egress.TrackOkType(), fdm.FailureType(), captypes.Converters...)
	if err != nil {
		return fmt.Errorf("constructing receipt reader: %w", err)
	}

	rcpt, err := rcptReader.Read(rcptLnk, blocks.Iterator())
	if err != nil {
		return fmt.Errorf("reading receipt: %w", err)
	}

	// we're not expecting any meaningful response here so we just check for error
	_, x := result.Unwrap(rcpt.Out())
	var emptyErr fdm.FailureModel
	if x != emptyErr {
		return fmt.Errorf("invocation failed: %s", x.Message)
	}

	return nil
}
