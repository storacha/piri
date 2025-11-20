package consolidationstore

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
)

const (
	trackInvocationsPrefix   = "track/"
	consolidateInvCIDsPrefix = "consolidate/"
)

// Store stores egress/track invocations and their corresponding
// consolidate invocation CIDs, indexed by batch CID.
// When a batch is tracked via space/egress/track, the receipt contains an effect
// with a space/egress/consolidate invocation. We store both the track invocation
// and the consolidate invocation CID for later retrieval.
type Store interface {
	// Put stores an egress/track invocation and consolidate invocation CID indexed by batch CID.
	Put(ctx context.Context, batchCID cid.Cid, trackInv invocation.Invocation, consolidateInvCID cid.Cid) error

	// GetTrackInvocation retrieves the egress/track invocation for a given batch CID.
	GetTrackInvocation(ctx context.Context, batchCID cid.Cid) (invocation.Invocation, error)

	// GetConsolidateInvocationCID retrieves the consolidate invocation CID for a given batch CID.
	GetConsolidateInvocationCID(ctx context.Context, batchCID cid.Cid) (cid.Cid, error)

	// Delete removes the track invocation and consolidate CID for a given batch CID.
	Delete(ctx context.Context, batchCID cid.Cid) error
}

type consolidationStore struct {
	trackInvocationsDS   datastore.Datastore
	consolidateInvCIDsDS datastore.Datastore
}

func (cs *consolidationStore) Put(ctx context.Context, batchCID cid.Cid, trackInv invocation.Invocation, consolidateInvCID cid.Cid) error {
	// Archive the invocation to CAR format
	b, err := io.ReadAll(trackInv.Archive())
	if err != nil {
		return fmt.Errorf("archiving track invocation: %w", err)
	}

	key := datastore.NewKey(batchCID.String())

	// Store track invocation
	if err := cs.trackInvocationsDS.Put(ctx, key, b); err != nil {
		return fmt.Errorf("writing track invocation to datastore: %w", err)
	}

	// Store consolidate invocation CID
	if err := cs.consolidateInvCIDsDS.Put(ctx, key, consolidateInvCID.Bytes()); err != nil {
		return fmt.Errorf("writing consolidate CID to datastore: %w", err)
	}

	return nil
}

func (cs *consolidationStore) GetTrackInvocation(ctx context.Context, batchCID cid.Cid) (invocation.Invocation, error) {
	key := datastore.NewKey(batchCID.String())

	data, err := cs.trackInvocationsDS.Get(ctx, key)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil, fmt.Errorf("track invocation not found for batch CID: %s", batchCID.String())
		}
		return nil, fmt.Errorf("getting %s from datastore: %w", batchCID, err)
	}

	inv, err := delegation.Extract(data)
	if err != nil {
		return nil, fmt.Errorf("extracting invocation: %w", err)
	}

	return inv, nil
}

func (cs *consolidationStore) GetConsolidateInvocationCID(ctx context.Context, batchCID cid.Cid) (cid.Cid, error) {
	key := datastore.NewKey(batchCID.String())

	data, err := cs.consolidateInvCIDsDS.Get(ctx, key)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return cid.Undef, fmt.Errorf("consolidate invocation CID not found for batch CID: %s", batchCID.String())
		}
		return cid.Undef, fmt.Errorf("getting %s from datastore: %w", batchCID, err)
	}

	// Parse CID from bytes
	c, err := cid.Cast(data)
	if err != nil {
		return cid.Undef, fmt.Errorf("parsing consolidate invocation CID: %w", err)
	}

	return c, nil
}

func (cs *consolidationStore) Delete(ctx context.Context, batchCID cid.Cid) error {
	key := datastore.NewKey(batchCID.String())

	// Delete track invocation
	if err := cs.trackInvocationsDS.Delete(ctx, key); err != nil && !errors.Is(err, datastore.ErrNotFound) {
		return fmt.Errorf("deleting track invocation from datastore: %w", err)
	}

	// Delete consolidate CID
	if err := cs.consolidateInvCIDsDS.Delete(ctx, key); err != nil && !errors.Is(err, datastore.ErrNotFound) {
		return fmt.Errorf("deleting consolidate CID from datastore: %w", err)
	}

	return nil
}

// New creates a [Store] backed by an IPFS datastore.
// The datastore is partitioned using prefixes for track invocations and consolidate CIDs.
func New(ds datastore.Datastore) Store {
	trackInvocationsDS := namespace.Wrap(ds, datastore.NewKey(trackInvocationsPrefix))
	consolidateInvCIDsDS := namespace.Wrap(ds, datastore.NewKey(consolidateInvCIDsPrefix))

	return &consolidationStore{
		trackInvocationsDS:   trackInvocationsDS,
		consolidateInvCIDsDS: consolidateInvCIDsDS,
	}
}
