package consolidationstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"

	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

const (
	minioTrackInvocationsPrefix   = "consolidation/track/"
	minioConsolidateInvCIDsPrefix = "consolidation/consolidate/"
)

// MinioConsolidationStore implements Store using a MinIO S3-compatible backend.
type MinioConsolidationStore struct {
	store             *minio.Store
	trackPrefix       string
	consolidatePrefix string
}

var _ Store = (*MinioConsolidationStore)(nil)

// NewMinioConsolidationStore creates a ConsolidationStore backed by a MinIO S3-compatible store.
// Track invocations are stored with prefix "consolidation/track/" and
// consolidate CIDs are stored with prefix "consolidation/consolidate/".
func NewMinioConsolidationStore(s *minio.Store) *MinioConsolidationStore {
	return &MinioConsolidationStore{
		store:             s,
		trackPrefix:       minioTrackInvocationsPrefix,
		consolidatePrefix: minioConsolidateInvCIDsPrefix,
	}
}

func (m *MinioConsolidationStore) Put(ctx context.Context, batchCID cid.Cid, trackInv invocation.Invocation, consolidateInvCID cid.Cid) error {
	// Archive the invocation to CAR format
	data, err := io.ReadAll(trackInv.Archive())
	if err != nil {
		return fmt.Errorf("archiving track invocation: %w", err)
	}

	trackKey := m.trackPrefix + batchCID.String() + ".car"
	consolidateKey := m.consolidatePrefix + batchCID.String() + ".ref"

	// Store track invocation
	if err := m.store.Put(ctx, trackKey, uint64(len(data)), bytes.NewReader(data)); err != nil {
		return fmt.Errorf("writing track invocation to minio: %w", err)
	}

	// Store consolidate invocation CID
	cidBytes := consolidateInvCID.Bytes()
	if err := m.store.Put(ctx, consolidateKey, uint64(len(cidBytes)), bytes.NewReader(cidBytes)); err != nil {
		return fmt.Errorf("writing consolidate CID to minio: %w", err)
	}

	return nil
}

func (m *MinioConsolidationStore) GetTrackInvocation(ctx context.Context, batchCID cid.Cid) (invocation.Invocation, error) {
	key := m.trackPrefix + batchCID.String() + ".car"

	obj, err := m.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil, fmt.Errorf("track invocation not found for batch CID: %s", batchCID.String())
		}
		return nil, fmt.Errorf("getting track invocation from minio: %w", err)
	}
	defer obj.Body().Close()

	data, err := io.ReadAll(obj.Body())
	if err != nil {
		return nil, fmt.Errorf("reading track invocation data: %w", err)
	}

	inv, err := delegation.Extract(data)
	if err != nil {
		return nil, fmt.Errorf("extracting invocation: %w", err)
	}

	return inv, nil
}

func (m *MinioConsolidationStore) GetConsolidateInvocationCID(ctx context.Context, batchCID cid.Cid) (cid.Cid, error) {
	key := m.consolidatePrefix + batchCID.String() + ".ref"

	obj, err := m.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return cid.Undef, fmt.Errorf("consolidate invocation CID not found for batch CID: %s", batchCID.String())
		}
		return cid.Undef, fmt.Errorf("getting consolidate CID from minio: %w", err)
	}
	defer obj.Body().Close()

	data, err := io.ReadAll(obj.Body())
	if err != nil {
		return cid.Undef, fmt.Errorf("reading consolidate CID data: %w", err)
	}

	// Parse CID from bytes
	c, err := cid.Cast(data)
	if err != nil {
		return cid.Undef, fmt.Errorf("parsing consolidate invocation CID: %w", err)
	}

	return c, nil
}

func (m *MinioConsolidationStore) Delete(ctx context.Context, batchCID cid.Cid) error {
	trackKey := m.trackPrefix + batchCID.String() + ".car"
	consolidateKey := m.consolidatePrefix + batchCID.String() + ".ref"

	// Delete track invocation (ignore not found errors)
	if err := m.store.Delete(ctx, trackKey); err != nil && !errors.Is(err, objectstore.ErrNotExist) {
		return fmt.Errorf("deleting track invocation from minio: %w", err)
	}

	// Delete consolidate CID (ignore not found errors)
	if err := m.store.Delete(ctx, consolidateKey); err != nil && !errors.Is(err, objectstore.ErrNotExist) {
		return fmt.Errorf("deleting consolidate CID from minio: %w", err)
	}

	return nil
}
