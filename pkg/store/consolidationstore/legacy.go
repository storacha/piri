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

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/consolidationstore/consolidation"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

// Legacy key prefixes for the old two-namespace storage format.
const (
	// Datastore prefixes
	legacyTrackPrefix       = "track/"
	legacyConsolidatePrefix = "consolidate/"

	// S3/MinIO prefixes
	legacyS3TrackPrefix       = "consolidation/track/"
	legacyS3ConsolidatePrefix = "consolidation/consolidate/"
)

// LegacyReader reads consolidations from the old two-namespace format.
type LegacyReader interface {
	// Get retrieves a consolidation from the legacy format.
	// Returns store.ErrNotFound if the consolidation does not exist.
	Get(ctx context.Context, batchCID cid.Cid) (consolidation.Consolidation, error)
	// Delete removes a consolidation from the legacy format.
	Delete(ctx context.Context, batchCID cid.Cid) error
}

// DatastoreLegacyReader reads from the old datastore two-namespace format.
type DatastoreLegacyReader struct {
	trackDS       datastore.Datastore
	consolidateDS datastore.Datastore
}

var _ LegacyReader = (*DatastoreLegacyReader)(nil)

// NewDatastoreLegacyReader creates a legacy reader for datastore backends.
// It wraps the datastore with the old namespace prefixes.
func NewDatastoreLegacyReader(ds datastore.Datastore) *DatastoreLegacyReader {
	return &DatastoreLegacyReader{
		trackDS:       namespace.Wrap(ds, datastore.NewKey(legacyTrackPrefix)),
		consolidateDS: namespace.Wrap(ds, datastore.NewKey(legacyConsolidatePrefix)),
	}
}

func (l *DatastoreLegacyReader) Get(ctx context.Context, batchCID cid.Cid) (consolidation.Consolidation, error) {
	key := datastore.NewKey(batchCID.String())

	// Read track invocation
	trackData, err := l.trackDS.Get(ctx, key)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return consolidation.Consolidation{}, store.ErrNotFound
		}
		return consolidation.Consolidation{}, fmt.Errorf("getting track invocation: %w", err)
	}

	trackInv, err := delegation.Extract(trackData)
	if err != nil {
		return consolidation.Consolidation{}, fmt.Errorf("extracting track invocation: %w", err)
	}

	// Read consolidate CID
	cidData, err := l.consolidateDS.Get(ctx, key)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return consolidation.Consolidation{}, store.ErrNotFound
		}
		return consolidation.Consolidation{}, fmt.Errorf("getting consolidate CID: %w", err)
	}

	consolidateCID, err := cid.Cast(cidData)
	if err != nil {
		return consolidation.Consolidation{}, fmt.Errorf("parsing consolidate CID: %w", err)
	}

	return consolidation.Consolidation{
		TrackInvocation:          trackInv,
		ConsolidateInvocationCID: consolidateCID,
	}, nil
}

func (l *DatastoreLegacyReader) Delete(ctx context.Context, batchCID cid.Cid) error {
	key := datastore.NewKey(batchCID.String())

	// Delete track invocation (ignore not found)
	if err := l.trackDS.Delete(ctx, key); err != nil && !errors.Is(err, datastore.ErrNotFound) {
		return fmt.Errorf("deleting track invocation: %w", err)
	}

	// Delete consolidate CID (ignore not found)
	if err := l.consolidateDS.Delete(ctx, key); err != nil && !errors.Is(err, datastore.ErrNotFound) {
		return fmt.Errorf("deleting consolidate CID: %w", err)
	}

	return nil
}

// S3LegacyReader reads from the old S3/MinIO two-prefix format.
type S3LegacyReader struct {
	store *minio.Store
}

var _ LegacyReader = (*S3LegacyReader)(nil)

// NewS3LegacyReader creates a legacy reader for S3/MinIO backends.
func NewS3LegacyReader(s *minio.Store) *S3LegacyReader {
	return &S3LegacyReader{store: s}
}

func (l *S3LegacyReader) Get(ctx context.Context, batchCID cid.Cid) (consolidation.Consolidation, error) {
	trackKey := legacyS3TrackPrefix + batchCID.String() + ".car"
	consolidateKey := legacyS3ConsolidatePrefix + batchCID.String() + ".ref"

	// Read track invocation
	trackObj, err := l.store.Get(ctx, trackKey)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return consolidation.Consolidation{}, store.ErrNotFound
		}
		return consolidation.Consolidation{}, fmt.Errorf("getting track invocation: %w", err)
	}
	defer trackObj.Body().Close()

	trackData, err := io.ReadAll(trackObj.Body())
	if err != nil {
		return consolidation.Consolidation{}, fmt.Errorf("reading track invocation: %w", err)
	}

	trackInv, err := delegation.Extract(trackData)
	if err != nil {
		return consolidation.Consolidation{}, fmt.Errorf("extracting track invocation: %w", err)
	}

	// Read consolidate CID
	cidObj, err := l.store.Get(ctx, consolidateKey)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return consolidation.Consolidation{}, store.ErrNotFound
		}
		return consolidation.Consolidation{}, fmt.Errorf("getting consolidate CID: %w", err)
	}
	defer cidObj.Body().Close()

	cidData, err := io.ReadAll(cidObj.Body())
	if err != nil {
		return consolidation.Consolidation{}, fmt.Errorf("reading consolidate CID: %w", err)
	}

	consolidateCID, err := cid.Cast(cidData)
	if err != nil {
		return consolidation.Consolidation{}, fmt.Errorf("parsing consolidate CID: %w", err)
	}

	return consolidation.Consolidation{
		TrackInvocation:          trackInv,
		ConsolidateInvocationCID: consolidateCID,
	}, nil
}

func (l *S3LegacyReader) Delete(ctx context.Context, batchCID cid.Cid) error {
	trackKey := legacyS3TrackPrefix + batchCID.String() + ".car"
	consolidateKey := legacyS3ConsolidatePrefix + batchCID.String() + ".ref"

	// Delete track invocation (ignore not found)
	if err := l.store.Delete(ctx, trackKey); err != nil && !errors.Is(err, objectstore.ErrNotExist) {
		return fmt.Errorf("deleting track invocation: %w", err)
	}

	// Delete consolidate CID (ignore not found)
	if err := l.store.Delete(ctx, consolidateKey); err != nil && !errors.Is(err, objectstore.ErrNotExist) {
		return fmt.Errorf("deleting consolidate CID: %w", err)
	}

	return nil
}
