package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"go.uber.org/zap/zapcore"
)

var (
	ErrNotExist = errors.New("object does not exist")
)

// ErrRangeNotSatisfiable is returned when the byte range option falls outside
// of the total size of the object.
type ErrRangeNotSatisfiable struct {
	Range Range
}

func (e ErrRangeNotSatisfiable) Error() string {
	var rangeStr string
	if e.Range.End != nil {
		rangeStr = fmt.Sprintf("%d-%d", e.Range.Start, *e.Range.End)
	} else {
		rangeStr = fmt.Sprintf("%d-", e.Range.Start)
	}
	return fmt.Sprintf("range not satisfiable: %s", rangeStr)
}

type Store interface {
	// Put stores an object with the given key and size from the provided reader.
	// The size parameter should match the actual bytes to be read from data.
	Put(ctx context.Context, key string, size uint64, data io.Reader) error
	// Get retrieves the object identified by the given key.
	// Returns an Object for reading the data, or an error if retrieval fails.
	// Use GetOption functions like WithRange to retrieve partial objects.
	Get(ctx context.Context, key string, opts ...GetOption) (Object, error)
	// Delete an object from the store by the given key.
	Delete(ctx context.Context, key string) error
}

// ListableStore extends Store with the ability to list objects by prefix
// and check for existence. This is implemented by stores that support
// efficient prefix-based queries (like S3/MinIO).
type ListableStore interface {
	Store
	// Exists checks if an object with the given key exists.
	Exists(ctx context.Context, key string) (bool, error)
	// ListPrefix returns an iterator over all object keys with the given prefix.
	// The iterator yields (key, error) pairs. Errors during listing are yielded
	// and iteration stops.
	ListPrefix(ctx context.Context, prefix string) iter.Seq2[string, error]
}

type Object interface {
	// Size returns the total size of the object in bytes.
	Size() int64
	Body() io.ReadCloser
}

type GetConfig interface {
	ProcessOptions([]GetOption)
	Range() Range
}

func NewGetConfig() GetConfig {
	return &options{}
}

type GetOption func(cfg *options)

type Range struct {
	// Start is the starting byte position (inclusive)
	Start uint64
	// End is the ending byte position (inclusive), nil means read to EOF
	End *uint64
}

type options struct {
	byteRange Range
}

func (o *options) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddUint64("start", o.byteRange.Start)
	if o.byteRange.End != nil {
		encoder.AddUint64("end", *o.byteRange.End)
	}
	return nil
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
// Start and End are inclusive byte positions, following HTTP range semantics.
// End can be nil to read from Start to EOF.
func WithRange(byteRange Range) GetOption {
	return func(opts *options) {
		opts.byteRange = byteRange
	}
}
