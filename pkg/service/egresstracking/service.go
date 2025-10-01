package egresstracking

import (
	"context"
	"fmt"
	"net/url"
	"sync"
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

	"github.com/storacha/piri/pkg/store/retrievaljournal"
)

// EgressTrackingService stores receipts from `space/content/retrieve` invocations and batches them.
// When batches reaches a certain size, they are sent to the egress tracking service via
// `space/egress/track` invocations.
type EgressTrackingService struct {
	mu                   sync.Mutex
	id                   principal.Signer
	egressTrackerDID     did.DID
	egressTrackerProofs  delegation.Proofs
	egressTrackerConn    client.Connection
	batchEndpoint        *url.URL
	store                retrievaljournal.Journal
	queue                EgressTrackingQueue
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
	queue EgressTrackingQueue,
	cleanupCheckInterval time.Duration,
) (*EgressTrackingService, error) {
	svc := &EgressTrackingService{
		id:                   id,
		egressTrackerDID:     egressTrackerConn.ID().DID(),
		egressTrackerProofs:  egressTrackerProofs,
		egressTrackerConn:    egressTrackerConn,
		batchEndpoint:        batchEndpoint,
		store:                store,
		queue:                queue,
		cleanupCheckInterval: cleanupCheckInterval,
		cleanupDone:          make(chan struct{}),
	}

	if err := queue.Register(svc.egressTrack); err != nil {
		return nil, fmt.Errorf("registering egress track task: %w", err)
	}

	return svc, nil
}

func (s *EgressTrackingService) AddReceipt(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	// we're not expecting any meaningful response here so we just check for error
	_, x := result.Unwrap(rcpt.Out())
	var emptyErr fdm.FailureModel
	if x != emptyErr {
		return fmt.Errorf("invocation failed: %s", x.Message)
	}

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
	defer ticker.Stop()

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
	batchCIDs, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing batches: %w", err)
	}

	if len(batchCIDs) == 0 {
		log.Debug("no batches to clean up")
		return nil
	}

	log.Debugf("checking %d batches for consolidation", len(batchCIDs))

	// Check each batch for consolidation
	for _, batchCID := range batchCIDs {
		if err := s.checkAndRemoveConsolidatedBatch(ctx, batchCID); err != nil {
			log.Errorf("error checking batch %s: %v", batchCID, err)
			// Continue with other batches even if one fails
		}
	}

	return nil
}

func (s *EgressTrackingService) checkAndRemoveConsolidatedBatch(ctx context.Context, batchCID cid.Cid) error {
	// Get the track invocation link for this batch to use as cause for consolidation
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
		return fmt.Errorf("creating track invocation for cause: %w", err)
	}

	// Request consolidation receipt from the egress tracker
	consolidateInv, err := egress.Consolidate.Invoke(
		s.id,
		s.egressTrackerDID,
		s.egressTrackerDID.String(),
		egress.ConsolidateCaveats{
			Cause: trackInv.Link(),
		},
		delegation.WithProof(s.egressTrackerProofs...),
		delegation.WithNoExpiration(),
	)
	if err != nil {
		return fmt.Errorf("creating consolidate invocation: %w", err)
	}

	resp, err := client.Execute(ctx, []invocation.Invocation{consolidateInv}, s.egressTrackerConn)
	if err != nil {
		return fmt.Errorf("executing consolidate invocation: %w", err)
	}

	rcptLnk, ok := resp.Get(consolidateInv.Link())
	if !ok {
		return fmt.Errorf("missing receipt for consolidate invocation: %s", consolidateInv.Link().String())
	}

	blocks, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(resp.Blocks()))
	if err != nil {
		return fmt.Errorf("importing response blocks into blockstore: %w", err)
	}

	rcptReader, err := egress.NewConsolidateReceiptReader()
	if err != nil {
		return fmt.Errorf("constructing consolidate receipt reader: %w", err)
	}

	rcpt, err := rcptReader.Read(rcptLnk, blocks.Iterator())
	if err != nil {
		return fmt.Errorf("reading consolidate receipt: %w", err)
	}

	// Check if the consolidation was successful
	okResult, errResult := result.Unwrap(rcpt.Out())
	var emptyErr egress.ConsolidateError
	if errResult != emptyErr {
		log.Warnf("batch %s not consolidated: %s", batchCID, errResult.Message)
		return nil
	}

	// Check if there were any errors in the consolidation
	if len(okResult.Errors) > 0 {
		log.Warnf("batch %s consolidated with %d errors, not removing", batchCID, len(okResult.Errors))
		return nil
	}

	// Consolidation was successful, remove the batch
	log.Infof("batch %s successfully consolidated, removing from store", batchCID)
	if err := s.store.Remove(ctx, batchCID); err != nil {
		return fmt.Errorf("removing consolidated batch: %w", err)
	}

	return nil
}
