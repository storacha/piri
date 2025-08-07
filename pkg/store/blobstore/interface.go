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

// GetOption is an option configuring byte retrieval from a blobstore.
type GetOption func(cfg *options) error

type Range struct {
	Offset uint64
	Length *uint64
}

type options struct {
	byteRange Range
}

func (o *options) ProcessOptions(opts []GetOption) {
	for _, opt := range opts {
		opt(o)
	}
}

func (o *options) Range() Range {
	return o.byteRange
}

// WithRange configures a byte range to extract.
func WithRange(byteRange Range) GetOption {
	return func(opts *options) error {
		opts.byteRange = byteRange
		return nil
	}
}

type Object interface {
	// Size returns the total size of the object in bytes.
	Size() int64
	Body() io.Reader
}

type Blobstore interface {
	// Put stores the bytes to the store and ensures it hashes to the passed
	// digest.
	Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error
	// Get retrieves the object identified by the passed digest. Returns nil and
	// [ErrNotFound] if the object does not exist.
	//
	// Note: data is not hashed on read.
	Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error)
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
	return &options{}
}
