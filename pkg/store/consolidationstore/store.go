package consolidationstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/consolidationstore/consolidation"
	"github.com/storacha/piri/pkg/store/genericstore"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/dsadapter"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

var log = logging.Logger("consolidationstore")

// Store stores egress/track invocations and their corresponding
// consolidate invocation CIDs, indexed by batch CID.
type Store interface {
	// Get retrieves the consolidation data for a given batch CID.
	// Returns store.ErrNotFound if the consolidation does not exist.
	Get(ctx context.Context, batchCID cid.Cid) (consolidation.Consolidation, error)
	// Put stores a consolidation indexed by batch CID.
	Put(ctx context.Context, batchCID cid.Cid, c consolidation.Consolidation) error
	// Delete removes the consolidation for a given batch CID.
	Delete(ctx context.Context, batchCID cid.Cid) error
}

// KeyEncoder defines how to encode keys for a specific backend.
type KeyEncoder interface {
	EncodeKey(batchCID cid.Cid) string
}

// S3KeyEncoder encodes keys for S3/MinIO backends.
type S3KeyEncoder struct{}

func (S3KeyEncoder) EncodeKey(batchCID cid.Cid) string {
	return batchCID.String() + ".cbor"
}

// DatastoreKeyEncoder encodes keys for LevelDB/datastore backends.
type DatastoreKeyEncoder struct{}

func (DatastoreKeyEncoder) EncodeKey(batchCID cid.Cid) string {
	return batchCID.String()
}

// consolidationStore implements Store backed by genericstore with legacy fallback.
type consolidationStore struct {
	store   *genericstore.Store[consolidation.Consolidation]
	encoder KeyEncoder
	legacy  LegacyReader
}

var _ Store = (*consolidationStore)(nil)

// New creates a ConsolidationStore with the given backend, prefix, key encoder, and legacy reader.
func New(backend objectstore.ListableStore, prefix string, encoder KeyEncoder, legacy LegacyReader) *consolidationStore {
	return &consolidationStore{
		store:   genericstore.New[consolidation.Consolidation](backend, prefix, consolidation.Codec{}),
		encoder: encoder,
		legacy:  legacy,
	}
}

func (s *consolidationStore) Get(ctx context.Context, batchCID cid.Cid) (consolidation.Consolidation, error) {
	// 1. Try new format first
	c, err := s.store.Get(ctx, s.encoder.EncodeKey(batchCID))
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return consolidation.Consolidation{}, fmt.Errorf("getting consolidation: %w", err)
	}

	// 2. Fall back to legacy format
	c, err = s.legacy.Get(ctx, batchCID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return consolidation.Consolidation{}, store.ErrNotFound
		}
		return consolidation.Consolidation{}, fmt.Errorf("getting legacy consolidation: %w", err)
	}

	// 3. Lazy migration: write to new format, delete from old
	if err := s.store.Put(ctx, s.encoder.EncodeKey(batchCID), c); err != nil {
		// Log but don't fail - we have the data
		log.Warnw("failed to migrate consolidation to new format", "batchCID", batchCID, "error", err)
		return c, nil
	}
	if err := s.legacy.Delete(ctx, batchCID); err != nil {
		log.Warnw("failed to delete legacy consolidation after migration", "batchCID", batchCID, "error", err)
	} else {
		log.Infow("migrated consolidation to new format", "batchCID", batchCID)
	}

	return c, nil
}

func (s *consolidationStore) Put(ctx context.Context, batchCID cid.Cid, c consolidation.Consolidation) error {
	// Always write to new format only
	return s.store.Put(ctx, s.encoder.EncodeKey(batchCID), c)
}

func (s *consolidationStore) Delete(ctx context.Context, batchCID cid.Cid) error {
	// Delete from new format
	newErr := s.store.Delete(ctx, s.encoder.EncodeKey(batchCID))

	// Also delete from legacy format to ensure cleanup
	legacyErr := s.legacy.Delete(ctx, batchCID)

	// Return error only if new format delete fails with non-NotFound error
	if newErr != nil && !errors.Is(newErr, store.ErrNotFound) {
		return fmt.Errorf("deleting consolidation: %w", newErr)
	}

	// If new format was not found but legacy also failed, return legacy error
	if newErr != nil && legacyErr != nil && !errors.Is(legacyErr, store.ErrNotFound) {
		return fmt.Errorf("deleting legacy consolidation: %w", legacyErr)
	}

	return nil
}

// NewS3Store creates a ConsolidationStore for S3/MinIO backends.
// Consolidations are stored with keys formatted as "consolidations/{batchCID}.cbor".
// Legacy data at "consolidation/track/" and "consolidation/consolidate/" is read and migrated.
func NewS3Store(backend *minio.Store) *consolidationStore {
	return New(
		backend,
		"consolidations/",
		S3KeyEncoder{},
		NewS3LegacyReader(backend),
	)
}

// NewDatastoreStore creates a ConsolidationStore for LevelDB/datastore backends.
// Consolidations are stored with keys formatted as "{batchCID}".
// Legacy data at "track/" and "consolidate/" namespaces is read and migrated.
func NewDatastoreStore(ds datastore.Datastore) *consolidationStore {
	return New(
		dsadapter.New(ds),
		"",
		DatastoreKeyEncoder{},
		NewDatastoreLegacyReader(ds),
	)
}
