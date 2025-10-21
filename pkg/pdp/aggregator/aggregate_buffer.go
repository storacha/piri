package aggregator

import (
	"context"
	// for go:embed
	_ "embed"
	"fmt"
	"sync"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"

	"github.com/storacha/piri/internal/ipldstore"
)

//go:embed aggregate_buffer.ipldsch
var bufferSchema []byte

var bufferTS *schema.TypeSystem

func init() {
	ts, err := ipld.LoadSchemaBytes(bufferSchema)
	if err != nil {
		panic(fmt.Errorf("loading submission buffer schema: %w", err))
	}
	bufferTS = ts
}

type Aggregates struct {
	Pending []datamodel.Link
}

// BufferStore provides persistent storage for submission state
type BufferStore interface {
	Aggregates(context.Context) (Aggregates, error)
	AppendAggregates(context.Context, []datamodel.Link) error
	ClearAggregates(context.Context) error
	NumAggregates(context.Context) (int, error)
}

// aggBufferKey is used as the single key for storing submission state
type aggBufferKey struct{}

func (aggBufferKey) String() string { return "aggregate_buffer" }

type submissionWorkspace struct {
	storeMu sync.RWMutex
	store   ipldstore.KVStore[aggBufferKey, Aggregates]
}

// NewSubmissionWorkspace creates a new submission workspace backed by the provided store
func NewSubmissionWorkspace(store store.SimpleStore) BufferStore {
	return &submissionWorkspace{
		store: ipldstore.IPLDStore[aggBufferKey, Aggregates](
			store,
			bufferTS.TypeByName("Aggregates"),
			types.Converters...,
		),
	}
}

// GetBuffer retrieves the current submission buffer state
func (sw *submissionWorkspace) Aggregates(ctx context.Context) (Aggregates, error) {
	buf, err := sw.store.Get(ctx, aggBufferKey{})
	if store.IsNotFound(err) {
		// Initialize empty buffer
		emptyBuffer := Aggregates{
			Pending: []datamodel.Link{},
		}
		if err := sw.store.Put(ctx, aggBufferKey{}, emptyBuffer); err != nil {
			return emptyBuffer, fmt.Errorf("initializing submission buffer: %w", err)
		}
		return emptyBuffer, nil
	}
	if err != nil {
		return Aggregates{}, fmt.Errorf("reading submission buffer: %w", err)
	}
	return buf, nil
}

// PutBuffer updates the submission buffer state
func (sw *submissionWorkspace) writeBuffer(ctx context.Context, buffer Aggregates) error {
	return sw.store.Put(ctx, aggBufferKey{}, buffer)
}

// ClearBuffer resets the submission buffer to empty state
func (sw *submissionWorkspace) ClearBuffer(ctx context.Context) error {
	return sw.store.Put(ctx, aggBufferKey{}, Aggregates{
		Pending: []datamodel.Link{},
	})
}

// AppendAggregates atomically appends new aggregates to the buffer
func (sw *submissionWorkspace) AppendAggregates(ctx context.Context, aggregates []datamodel.Link) error {
	if len(aggregates) == 0 {
		return nil
	}

	buffer, err := sw.Aggregates(ctx)
	if err != nil {
		return fmt.Errorf("getting buffer for append: %w", err)
	}

	buffer.Pending = append(buffer.Pending, aggregates...)

	if err := sw.writeBuffer(ctx, buffer); err != nil {
		return fmt.Errorf("saving buffer after append: %w", err)
	}

	return nil
}

// ClearAggregates atomically clears the pending aggregates while preserving other state
func (sw *submissionWorkspace) ClearAggregates(ctx context.Context) error {
	return sw.store.Put(ctx, aggBufferKey{}, Aggregates{
		Pending: []datamodel.Link{},
	})
}

// BufferSize returns the number of pending aggregates without loading full buffer data
func (sw *submissionWorkspace) NumAggregates(ctx context.Context) (int, error) {
	buffer, err := sw.Aggregates(ctx)
	if err != nil {
		if store.IsNotFound(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("getting buffer size: %w", err)
	}

	return len(buffer.Pending), nil
}
