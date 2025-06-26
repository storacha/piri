package objectstore

import (
	"context"
	"io"

	"go.uber.org/zap/zapcore"
)

type Store interface {
	// Put stores an object with the given key and size from the provided reader.
	// The size parameter should match the actual bytes to be read from data.
	Put(ctx context.Context, key string, size uint64, data io.Reader) error
	// Get retrieves the object identified by the given key.
	// Returns an Object for reading the data, or an error if retrieval fails.
	// Use GetOption functions like WithRange to retrieve partial objects.
	Get(ctx context.Context, key string, opts ...GetOption) (Object, error)
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
