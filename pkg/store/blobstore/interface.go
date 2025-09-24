package blobstore

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/multiformats/go-multihash"
)

// ErrDataInconsistent is returned when the data being written does not hash to
// the expected value.
var ErrDataInconsistent = errors.New("data consistency check failed")

// ErrTooLarge is returned when the data being written is larger than expected.
var ErrTooLarge = errors.New("payload too large")

// ErrTooSmall is returned when the data being written is smaller than expected.
var ErrTooSmall = errors.New("payload too small")

// ErrRangeNotSatisfiable is returned when the byte range option falls outside
// of the total size of the blob.
var ErrRangeNotSatisfiable = errors.New("range not satisfiable")

// GetOption is an option configuring byte retrieval from a blobstore.
type GetOption func(cfg *GetOptions) error

type Range struct {
	// Start is the byte to start extracting from (inclusive).
	Start uint64
	// End is the byte to stop extracting at (inclusive).
	End *uint64
}

type GetOptions struct {
	ByteRange Range
}

func (o *GetOptions) ProcessOptions(opts []GetOption) {
	for _, opt := range opts {
		opt(o)
	}
}

func (o *GetOptions) Range() Range {
	return o.ByteRange
}

// WithRange configures a byte range to extract.
func WithRange(start uint64, end *uint64) GetOption {
	return func(opts *GetOptions) error {
		opts.ByteRange = Range{start, end}
		return nil
	}
}

type Object interface {
	// Size returns the total size of the object in bytes.
	Size() int64
	Body() io.ReadCloser
}

type BlobGetter interface {
	// Get retrieves the object identified by the passed digest. Returns nil and
	// [ErrNotFound] if the object does not exist.
	//
	// Note: data is not hashed on read.
	Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error)
}

type Blobstore interface {
	BlobGetter
	// Put stores the bytes to the store and ensures it hashes to the passed
	// digest.
	Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error
}

// FileSystemer exposes the filesystem interface for reading blobs.
type FileSystemer interface {
	// FileSystem returns a filesystem interface for reading blobs.
	FileSystem() http.FileSystem
}

type PDPStore interface {
	Blobstore
	FileSystemer
}

type GetConfig interface {
	ProcessOptions([]GetOption)
	Range() Range
}

func NewGetConfig() GetConfig {
	return &GetOptions{}
}
