package egresstracking

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

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

	"github.com/storacha/piri/pkg/client/receipts"
	"github.com/storacha/piri/pkg/store/consolidationstore"
	"github.com/storacha/piri/pkg/store/retrievaljournal"
)

// EgressTrackingService stores receipts from `space/content/retrieve` invocations and batches them.
// When batches reaches a certain size, they are sent to the egress tracking service via
// `space/egress/track` invocations.
type EgressTrackingService struct {
	id                   principal.Signer
	egressTrackerDID     did.DID
	egressTrackerProofs  delegation.Proofs
	egressTrackerConn    client.Connection
	batchEndpoint        *url.URL
	journal              retrievaljournal.Journal
	queue                EgressTrackingQueue
	consolidationStore   consolidationstore.Store
	rcptsClient          *receipts.Client
	cleanupCheckInterval time.Duration
	cleanupCancel        context.CancelFunc
	cleanupDone          chan struct{}
}

func New(
	id principal.Signer,
	egressTrackerConn client.Connection,
	egressTrackerProofs delegation.Proofs,
	batchEndpoint *url.URL,
	store retrievaljournal.Journal,
	consolidationStore consolidationstore.Store,
	queue EgressTrackingQueue,
	rcptsClient *receipts.Client,
	cleanupCheckInterval time.Duration,
) (*EgressTrackingService, error) {
	svc := &EgressTrackingService{
		id:                   id,
		egressTrackerDID:     egressTrackerConn.ID().DID(),
		egressTrackerProofs:  egressTrackerProofs,
		egressTrackerConn:    egressTrackerConn,
		batchEndpoint:        batchEndpoint,
		journal:              store,
		consolidationStore:   consolidationStore,
		queue:                queue,
		rcptsClient:          rcptsClient,
		cleanupCheckInterval: cleanupCheckInterval,
		cleanupDone:          make(chan struct{}),
	}

	if err := queue.Register(svc.egressTrack); err != nil {
		return nil, fmt.Errorf("registering egress track task: %w", err)
	}

	return svc, nil
}

func (s *EgressTrackingService) AddReceipt(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) error {
	batchRotated, rotatedBatchCID, err := s.journal.Append(ctx, rcpt)
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

func (s *EgressTrackingService) enqueueEgressTrackTask(ctx context.Context, batchCID cid.Cid) error {
	return s.queue.Enqueue(ctx, batchCID)
}

func (s *EgressTrackingService) egressTrack(ctx context.Context, batchCID cid.Cid) error {
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

	_, x := result.Unwrap(rcpt.Out())
	var emptyErr fdm.FailureModel
	if x != emptyErr {
		return fmt.Errorf("invocation failed: %s", x.Message)
	}

	// Extract the consolidate invocation from the receipt's effects
	effects := rcpt.Fx()
	if effects == nil {
		return fmt.Errorf("receipt has no effects")
	}

	forks := effects.Fork()
	if len(forks) != 1 {
		return fmt.Errorf("expected exactly one fork effect, but got: %d", len(forks))
	}

	consolidateInvLink := forks[0].Link()

	// Store the track invocation and consolidate CID (indexed by batch CID)
	consolidateCID := consolidateInvLink.(cidlink.Link).Cid
	if err := s.consolidationStore.Put(ctx, batchCID, trackInv, consolidateCID); err != nil {
		return fmt.Errorf("storing track invocation in consolidation store: %w", err)
	}
	log.Infof("stored track invocation with consolidate invocation %s for batch %s", consolidateInvLink.String(), batchCID.String())

	return nil
}

// StartCleanupTask starts the periodic cleanup task that checks for consolidated batches
// and removes them from the store.
func (s *EgressTrackingService) StartCleanupTask(ctx context.Context) error {
	if s.cleanupCheckInterval <= 0 {
		log.Info("cleanup task disabled (interval is 0)")
		close(s.cleanupDone)
		return nil
	}

	cleanupCtx, cancel := context.WithCancel(ctx)
	s.cleanupCancel = cancel

	go s.runCleanupTask(cleanupCtx)

	log.Infof("cleanup task started with interval: %v", s.cleanupCheckInterval)
	return nil
}

// StopCleanupTask stops the periodic cleanup task gracefully.
func (s *EgressTrackingService) StopCleanupTask(ctx context.Context) error {
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}

	select {
	case <-s.cleanupDone:
		log.Info("cleanup task stopped")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for cleanup task to stop: %w", ctx.Err())
	}
}

func (s *EgressTrackingService) runCleanupTask(ctx context.Context) {
	defer close(s.cleanupDone)

	ticker := time.NewTicker(s.cleanupCheckInterval)

	for {
		select {
		case <-ctx.Done():
			log.Info("cleanup task context cancelled")
			return
		case <-ticker.C:
			if err := s.cleanupConsolidatedBatches(ctx); err != nil {
				log.Errorf("error cleaning up consolidated batches: %v", err)
			}
		}
	}
}

func (s *EgressTrackingService) cleanupConsolidatedBatches(ctx context.Context) error {
	// List all batches
	batchCIDs, err := s.journal.List(ctx)
	if err != nil {
		return fmt.Errorf("listing batches: %w", err)
	}

	// Check each batch for consolidation
	// TODO: consider doing this in parallel
	for batchCID := range batchCIDs {
		if err := s.checkAndRemoveConsolidatedBatch(ctx, batchCID); err != nil {
			log.Errorf("error checking batch %s: %v", batchCID, err)
			// Continue with other batches even if one fails
		}
	}

	return nil
}

func (s *EgressTrackingService) checkAndRemoveConsolidatedBatch(ctx context.Context, batchCID cid.Cid) error {
	// Get the consolidate invocation CID from the consolidation store
	consolidateInvCID, err := s.consolidationStore.GetConsolidateInvocationCID(ctx, batchCID)
	if err != nil {
		log.Warnf("batch %s not found in consolidation store, skipping: %v", batchCID, err)
		return nil
	}

	// Fetch the consolidate receipt from the egress tracker's receipts endpoint
	rcpt, err := s.rcptsClient.Fetch(ctx, cidlink.Link{Cid: consolidateInvCID})
	if err != nil {
		if errors.Is(err, receipts.ErrNotFound) {
			log.Debugf("consolidate receipt not yet available for batch %s", batchCID)
			return nil
		}

		return fmt.Errorf("fetching consolidate receipt: %w", err)
	}

	log.Debugf("consolidate receipt fetched for batch %s", batchCID.String())

	// Fetch the original track invocation for the batch
	trackInv, err := s.consolidationStore.GetTrackInvocation(ctx, batchCID)
	if err != nil {
		return fmt.Errorf("batch %s not found in consolidation store: %w", batchCID, err)
	}

	if err := s.validateConsolidateReceipt(rcpt, trackInv); err != nil {
		return fmt.Errorf("receipt failed validation: %w", err)
	}

	// Remove the batch from the store
	log.Infof("consolidate receipt found for batch %s, removing from store", batchCID)
	if err := s.journal.Remove(ctx, batchCID); err != nil {
		return fmt.Errorf("removing consolidated batch: %w", err)
	}

	// Remove from consolidation store
	if err := s.consolidationStore.Delete(ctx, batchCID); err != nil {
		log.Warnf("failed to remove batch %s from consolidation store: %v", batchCID, err)
	}

	log.Debugf("batch %s removed from journal and consolidation store", batchCID.String())

	return nil
}

func (s *EgressTrackingService) validateConsolidateReceipt(receipt receipt.AnyReceipt, trackInv invocation.Invocation) error {
	// TODO: Validate the receipt. This will include checking the receipt matches the original track invocation
	// and confirming that the consolidated amount of bytes matches our records.
	return nil
}
