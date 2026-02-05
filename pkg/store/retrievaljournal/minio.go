package retrievaljournal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"iter"
	"strings"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	carutil "github.com/ipld/go-car/util"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/failure"
	"github.com/storacha/go-ucanto/core/receipt"

	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

const (
	minioBatchPrefix = "journal/batches/"
)

var _ Journal = (*minioJournal)(nil)

// minioJournal implements Journal using MinIO S3-compatible storage.
// The work-in-progress batch is buffered in memory and flushed to S3 on rotation.
// Completed batches are stored with keys like "journal/batches/{cid}.car".
type minioJournal struct {
	mu           sync.Mutex
	store        *minio.Store
	prefix       string
	wipBuffer    bytes.Buffer
	wipHash      hash.Hash
	maxBatchSize int64
	initialized  bool
}

// NewMinioJournal creates a new MinIO-backed retrieval journal.
// If maxBatchSize is 0, DefaultBatchSize will be used.
func NewMinioJournal(s *minio.Store, maxBatchSize int64) (*minioJournal, error) {
	if maxBatchSize <= 0 {
		maxBatchSize = DefaultBatchSize
	}

	j := &minioJournal{
		store:        s,
		prefix:       minioBatchPrefix,
		maxBatchSize: maxBatchSize,
		wipHash:      sha256.New(),
	}

	// Initialize the WIP batch with CAR header
	if err := j.initBatch(); err != nil {
		return nil, fmt.Errorf("initializing batch: %w", err)
	}

	return j, nil
}

func (j *minioJournal) initBatch() error {
	j.wipBuffer.Reset()
	j.wipHash = sha256.New()

	// Create a multiwriter to write to both buffer and hash
	multiw := io.MultiWriter(&j.wipBuffer, j.wipHash)

	// Write the CAR header
	hdr := &car.CarHeader{Roots: []cid.Cid{}, Version: 1}
	if err := car.WriteHeader(hdr, multiw); err != nil {
		return fmt.Errorf("writing CAR header: %w", err)
	}

	j.initialized = true
	return nil
}

func (j *minioJournal) Append(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, failure.FailureModel]) (bool, cid.Cid, error) {
	if rcpt == nil {
		return false, cid.Cid{}, fmt.Errorf("receipt is nil")
	}

	rcptArchive := rcpt.Archive()
	archiveBytes, err := io.ReadAll(rcptArchive)
	if err != nil {
		return false, cid.Cid{}, fmt.Errorf("reading receipt archive: %w", err)
	}

	archiveCID, err := cid.V1Builder{
		Codec:  uint64(multicodec.Car),
		MhType: uint64(multihash.SHA2_256),
	}.Sum(archiveBytes)
	if err != nil {
		return false, cid.Cid{}, fmt.Errorf("creating receipt archive CID: %w", err)
	}

	cidBytes := archiveCID.Bytes()

	j.mu.Lock()
	defer j.mu.Unlock()

	if !j.initialized {
		if err := j.initBatch(); err != nil {
			return false, cid.Cid{}, fmt.Errorf("initializing batch: %w", err)
		}
	}

	// Create multiwriter for both buffer and hash
	multiw := io.MultiWriter(&j.wipBuffer, j.wipHash)

	// Append the block using CAR LD format
	if err := carutil.LdWrite(multiw, cidBytes, archiveBytes); err != nil {
		return false, cid.Cid{}, fmt.Errorf("writing to batch: %w", err)
	}

	// Check if we should rotate
	if int64(j.wipBuffer.Len()) >= j.maxBatchSize {
		rotatedBatchCID, err := j.rotate(ctx)
		if err != nil {
			return false, cid.Cid{}, fmt.Errorf("rotating batch: %w", err)
		}
		return true, rotatedBatchCID, nil
	}

	return false, cid.Cid{}, nil
}

func (j *minioJournal) rotate(ctx context.Context) (cid.Cid, error) {
	// Compute the CID of the batch from the hash
	mhBytes, _ := multihash.Encode(j.wipHash.Sum(nil), multihash.SHA2_256)
	mh := multihash.Multihash(mhBytes)
	batchCID := cid.NewCidV1(uint64(multicodec.Car), mh)

	// Upload to S3
	key := j.prefix + batchCID.String() + ".car"
	data := j.wipBuffer.Bytes()
	if err := j.store.Put(ctx, key, uint64(len(data)), bytes.NewReader(data)); err != nil {
		return cid.Cid{}, fmt.Errorf("uploading batch to minio: %w", err)
	}

	log.Infow("rotated batch to minio", "cid", batchCID.String(), "size", len(data))

	// Reset for new batch
	if err := j.initBatch(); err != nil {
		return cid.Cid{}, fmt.Errorf("initializing new batch: %w", err)
	}

	return batchCID, nil
}

func (j *minioJournal) GetBatch(ctx context.Context, c cid.Cid) (io.ReadCloser, error) {
	key := j.prefix + c.String() + ".car"
	obj, err := j.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil, fmt.Errorf("batch not found: %s", c.String())
		}
		return nil, fmt.Errorf("getting batch from minio: %w", err)
	}
	return obj.Body(), nil
}

func (j *minioJournal) List(ctx context.Context) (iter.Seq[cid.Cid], error) {
	return func(yield func(cid.Cid) bool) {
		for key, err := range j.store.ListPrefix(ctx, j.prefix) {
			if err != nil {
				log.Warnf("error listing batches: %v", err)
				return
			}

			// Extract CID from key: prefix + {cid}.car
			name := strings.TrimPrefix(key, j.prefix)
			if !strings.HasSuffix(name, ".car") {
				continue
			}

			cidStr := strings.TrimSuffix(name, ".car")
			c, err := cid.Decode(cidStr)
			if err != nil {
				log.Warnf("skipping key with invalid CID: %s: %v", key, err)
				continue
			}

			if !yield(c) {
				return
			}
		}
	}, nil
}

func (j *minioJournal) Remove(ctx context.Context, c cid.Cid) error {
	key := j.prefix + c.String() + ".car"
	if err := j.store.Delete(ctx, key); err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil // Already removed, not an error
		}
		return fmt.Errorf("removing batch from minio: %w", err)
	}
	return nil
}

// Flush forces the current WIP batch to be rotated and uploaded to S3,
// even if it hasn't reached the size limit. This is useful for graceful shutdown.
// Returns the CID of the flushed batch, or cid.Undef if the batch was empty.
func (j *minioJournal) Flush(ctx context.Context) (cid.Cid, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if !j.initialized {
		return cid.Undef, nil
	}

	// Check if there's any data beyond the CAR header
	// A CAR header is typically ~20 bytes, so if buffer is small, it's empty
	hdr := &car.CarHeader{Roots: []cid.Cid{}, Version: 1}
	hdrSize, _ := car.HeaderSize(hdr)
	if j.wipBuffer.Len() <= int(hdrSize) {
		return cid.Undef, nil
	}

	return j.rotate(ctx)
}

// Close flushes any pending data and releases resources.
func (j *minioJournal) Close() error {
	// Attempt to flush, but don't fail if there's nothing to flush
	_, _ = j.Flush(context.Background())
	return nil
}
