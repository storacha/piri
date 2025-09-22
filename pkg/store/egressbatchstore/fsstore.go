package egressbatchstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/v2/blockstore"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/receipt"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
)

const (
	// DefaultBatchSize is the default maximum size of a receipt batch in bytes.
	DefaultBatchSize = 100 * 1024 * 1024 // 100MiB

	// currentBatchName is the name of the current batch file.
	currentBatchName = "egress.car.wip"

	// batchFilePrefix is the prefix for completed batch files.
	batchFilePrefix = "egress."
	batchFileSuffix = ".car"
)

var _ EgressBatchStore = (*fsBatchStore)(nil)

type fsBatchStore struct {
	basePath     string
	curBatchPath string
	maxBatchSize int64
}

// NewFSBatchStore creates a new file system based batch receipt store.
// Batches will be stored in the given basePath.
// If maxBatchSize is 0, DefaultBatchSize will be used.
func NewFSBatchStore(basePath string, maxBatchSize int64) (*fsBatchStore, error) {
	if maxBatchSize <= 0 {
		maxBatchSize = DefaultBatchSize
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("creating egress batch store directory: %w", err)
	}

	curBatchPath := filepath.Join(basePath, currentBatchName)

	return &fsBatchStore{
		basePath:     basePath,
		curBatchPath: curBatchPath,
		maxBatchSize: maxBatchSize,
	}, nil
}

func (s *fsBatchStore) Append(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) (bool, cid.Cid, error) {
	if rcpt == nil {
		return false, cid.Cid{}, fmt.Errorf("receipt is nil")
	}

	rwbs, err := blockstore.OpenReadWrite(s.curBatchPath, nil)
	if err != nil {
		return false, cid.Cid{}, fmt.Errorf("opening current batch for writing: %w", err)
	}

	rcptArchive := rcpt.Archive()
	archiveBytes, err := io.ReadAll(rcptArchive)
	if err != nil {
		return false, cid.Cid{}, fmt.Errorf("reading receipt archive: %w", err)
	}

	archiveCID, err := cid.V1Builder{
		Codec:    uint64(multicodec.Car),
		MhType:   uint64(multihash.SHA2_256),
		MhLength: 0,
	}.Sum(archiveBytes)
	if err != nil {
		return false, cid.Cid{}, fmt.Errorf("creating receipt archive CID: %w", err)
	}

	block, err := blocks.NewBlockWithCid(archiveBytes, archiveCID)
	if err != nil {
		return false, cid.Cid{}, fmt.Errorf("creating receipt block: %w", err)
	}

	if err := rwbs.Put(ctx, block); err != nil {
		return false, cid.Cid{}, fmt.Errorf("adding receipt block to batch: %w", err)
	}

	if err := rwbs.Finalize(); err != nil {
		return false, cid.Cid{}, fmt.Errorf("finalizing batch: %w", err)
	}

	// rotate the batch if it exceeds the size limit
	curSize, err := s.currentBatchSize()
	if err != nil {
		return false, cid.Cid{}, fmt.Errorf("checking current batch size: %w", err)
	}
	if curSize >= s.maxBatchSize {
		rotatedBatchCID, err := s.rotate()
		if err != nil {
			return false, cid.Cid{}, fmt.Errorf("rotating batch: %w", err)
		}

		return true, rotatedBatchCID, nil
	}

	return false, cid.Cid{}, nil
}

func (s *fsBatchStore) currentBatchSize() (int64, error) {
	info, err := os.Stat(s.curBatchPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}

		return 0, fmt.Errorf("checking current batch file: %w", err)
	}

	return info.Size(), nil
}

func (s *fsBatchStore) rotate() (cid.Cid, error) {
	// Calculate the CID of the current batch
	f, err := os.Open(s.curBatchPath)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("opening current batch file: %w", err)
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return cid.Cid{}, fmt.Errorf("hashing batch file: %w", err)
	}

	// error from Encode can be discarded, it's always nil
	mhBytes, _ := multihash.Encode(hash.Sum(nil), multihash.SHA2_256)
	mh := multihash.Multihash(mhBytes)

	batchCID := cid.NewCidV1(uint64(multicodec.Car), mh)
	newPath := filepath.Join(s.basePath, batchFilePrefix+batchCID.String()+batchFileSuffix)

	// Rename the file to include the CID
	if err := os.Rename(s.curBatchPath, newPath); err != nil {
		return cid.Cid{}, fmt.Errorf("renaming batch file: %w", err)
	}

	return batchCID, nil
}

func (s *fsBatchStore) GetBatch(ctx context.Context, cid cid.Cid) (reader io.ReadCloser, err error) {
	return os.Open(filepath.Join(s.basePath, batchFilePrefix+cid.String()+batchFileSuffix))
}
