package retrievaljournal

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-car"
	carutil "github.com/ipld/go-car/util"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/receipt"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
)

var log = logging.Logger("retrievaljournal")

const (
	// DefaultBatchSize is the default maximum size of a receipt batch in bytes.
	DefaultBatchSize = 100 * 1024 * 1024 // 100MiB

	// currentBatchName is the name of the current batch file.
	currentBatchName = "egress.car.wip"

	// batchFilePrefix is the prefix for completed batch files.
	batchFilePrefix = "egress."
	batchFileSuffix = ".car"
)

var _ Journal = (*fsJournal)(nil)

type fsJournal struct {
	mu            sync.Mutex
	basePath      string
	currBatchPath string
	currBatch     *os.File
	currSize      int64
	maxBatchSize  int64
}

// NewFSJournal creates a new file system based retrieval journal.
// Batches will be stored in the given basePath.
// If maxBatchSize is 0, DefaultBatchSize will be used.
func NewFSJournal(basePath string, maxBatchSize int64) (*fsJournal, error) {
	if maxBatchSize <= 0 {
		maxBatchSize = DefaultBatchSize
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("creating retrieval journal directory: %w", err)
	}

	currBatchPath := filepath.Join(basePath, currentBatchName)

	j := &fsJournal{
		basePath:      basePath,
		currBatchPath: currBatchPath,
		maxBatchSize:  maxBatchSize,
	}

	if err := j.newBatch(false); err != nil {
		return nil, fmt.Errorf("creating or opening current batch file: %w", err)
	}

	return j, nil
}

func (j *fsJournal) newBatch(truncate bool) error {
	flags := os.O_RDWR | os.O_CREATE
	if truncate {
		flags |= os.O_TRUNC
	}

	var err error
	j.currBatch, err = os.OpenFile(j.currBatchPath, flags, 0644)
	if err != nil {
		return err
	}

	if truncate {
		j.currSize = 0
	} else {
		info, err := j.currBatch.Stat()
		if err != nil {
			return err
		}

		j.currSize = info.Size()
	}

	if j.currSize == 0 {
		// Write the CAR header if the file is new or truncated
		hdr := &car.CarHeader{Roots: []cid.Cid{}, Version: 1}
		if err := car.WriteHeader(hdr, j.currBatch); err != nil {
			return err
		}

		hdrSize, err := car.HeaderSize(hdr)
		if err != nil {
			return err
		}

		j.currSize = int64(hdrSize)
	}

	return nil
}

func (j *fsJournal) Append(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) (bool, cid.Cid, error) {
	if rcpt == nil {
		return false, cid.Cid{}, fmt.Errorf("receipt is nil")
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

	// cid to bytes
	cidBytes := archiveCID.Bytes()

	j.mu.Lock()
	defer j.mu.Unlock()

	// append a line in the car file, this is what `Put` is doing internally, but less complicated.
	if err := carutil.LdWrite(j.currBatch, cidBytes, archiveBytes); err != nil {
		return false, cid.Cid{}, err
	}
	// record the size of the data written
	blockSize := carutil.LdSize(cidBytes, archiveBytes)
	j.currSize += int64(blockSize)

	// rotate the batch if it exceeds the size limit
	if j.currSize >= j.maxBatchSize {
		rotatedBatchCID, err := j.rotate()
		if err != nil {
			return false, cid.Cid{}, fmt.Errorf("rotating batch: %w", err)
		}

		return true, rotatedBatchCID, nil
	}

	return false, cid.Cid{}, nil
}

func (j *fsJournal) rotate() (cid.Cid, error) {
	// Reuse the open file handle to calculate the hash
	off, err := j.currBatch.Seek(0, io.SeekStart)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("seeking to start of batch file: %w", err)
	}
	if off != 0 {
		return cid.Cid{}, fmt.Errorf("failed to seek to start of batch file")
	}

	// Calculate the CID of the current batch
	hash := sha256.New()
	n, err := io.Copy(hash, j.currBatch)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("hashing batch file: %w", err)
	}
	if n != j.currSize {
		return cid.Cid{}, fmt.Errorf("expected to copy %d bytes, but got %d", n, j.currSize)
	}

	// Close the current batch file
	if err := j.currBatch.Close(); err != nil {
		return cid.Cid{}, fmt.Errorf("closing current batch file: %w", err)
	}

	// error from Encode can be discarded, it's always nil
	mhBytes, _ := multihash.Encode(hash.Sum(nil), multihash.SHA2_256)
	mh := multihash.Multihash(mhBytes)

	batchCID := cid.NewCidV1(uint64(multicodec.Car), mh)

	// Rename the file to include the CID
	newPath := filepath.Join(j.basePath, batchFilePrefix+batchCID.String()+batchFileSuffix)
	if err := os.Rename(j.currBatchPath, newPath); err != nil {
		return cid.Cid{}, fmt.Errorf("renaming batch file: %w", err)
	}

	// Create a new current batch file
	if err := j.newBatch(true); err != nil {
		return cid.Cid{}, fmt.Errorf("creating new batch file: %w", err)
	}

	return batchCID, nil
}

func (j *fsJournal) GetBatch(ctx context.Context, cid cid.Cid) (reader io.ReadCloser, err error) {
	return os.Open(filepath.Join(j.basePath, batchFilePrefix+cid.String()+batchFileSuffix))
}

func (s *fsJournal) List(ctx context.Context) ([]cid.Cid, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("reading batch directory: %w", err)
	}

	var cids []cid.Cid
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip the current batch file
		if name == currentBatchName {
			continue
		}

		// Check if the file has the correct prefix and suffix
		if !strings.HasPrefix(name, batchFilePrefix) || !strings.HasSuffix(name, batchFileSuffix) {
			continue
		}

		// Extract the CID from the filename
		cidStr := name[len(batchFilePrefix) : len(name)-len(batchFileSuffix)]
		c, err := cid.Decode(cidStr)
		if err != nil {
			log.Warnf("skipping file with invalid CID in name: %s: %v", name, err)
			continue
		}

		cids = append(cids, c)
	}

	return cids, nil
}

func (s *fsJournal) Remove(ctx context.Context, cid cid.Cid) error {
	path := filepath.Join(s.basePath, batchFilePrefix+cid.String()+batchFileSuffix)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing batch file: %w", err)
	}
	return nil
}

func (j *fsJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.currBatch.Close()
}
