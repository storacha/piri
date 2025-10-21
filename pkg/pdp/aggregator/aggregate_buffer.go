package aggregator

import (
	"context"
	// for go:embed
	_ "embed"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"go.uber.org/fx"

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
}

// aggBufferKey is used as the single key for storing submission state
type aggBufferKey struct{}

func (aggBufferKey) String() string { return "aggregate_buffer" }

type submissionWorkspace struct {
	storeMu sync.RWMutex
	store   ipldstore.KVStore[aggBufferKey, Aggregates]
}

type SubmissionWorkspaceParams struct {
	fx.In
	Datastore datastore.Datastore `name:"aggregator_datastore"`
}

const ManagerKey = "manager/"

// NewSubmissionWorkspace creates a new submission workspace backed by the provided store
func NewSubmissionWorkspace(params SubmissionWorkspaceParams) (BufferStore, error) {
	ss := store.SimpleStoreFromDatastore(namespace.Wrap(params.Datastore, datastore.NewKey(ManagerKey)))
	sw := &submissionWorkspace{
		store: ipldstore.IPLDStore[aggBufferKey, Aggregates](
			ss,
			bufferTS.TypeByName("Aggregates"),
			types.Converters...,
		),
	}

	// Initialize empty buffer at creation time to avoid race conditions
	// and side effects in read operations
	ctx := context.Background()
	emptyBuffer := Aggregates{
		Pending: []datamodel.Link{},
	}
	err := sw.store.Put(ctx, aggBufferKey{}, emptyBuffer)
	if err != nil {
		return nil, fmt.Errorf("putting empty buffer: %w", err)
	}

	return sw, nil
}

// Aggregates retrieves the current submission buffer state
func (sw *submissionWorkspace) Aggregates(ctx context.Context) (Aggregates, error) {
	sw.storeMu.RLock()
	defer sw.storeMu.RUnlock()

	buf, err := sw.store.Get(ctx, aggBufferKey{})
	if err != nil {
		// If not found, return empty aggregates (should not happen after initialization)
		if store.IsNotFound(err) {
			return Aggregates{
				Pending: []datamodel.Link{},
			}, nil
		}
		return Aggregates{}, fmt.Errorf("reading submission buffer: %w", err)
	}
	return buf, nil
}

// AppendAggregates atomically appends new aggregates to the buffer
func (sw *submissionWorkspace) AppendAggregates(ctx context.Context, aggregates []datamodel.Link) error {
	if len(aggregates) == 0 {
		return nil
	}

	sw.storeMu.Lock()
	defer sw.storeMu.Unlock()

	buffer, err := sw.store.Get(ctx, aggBufferKey{})
	if err != nil {
		return fmt.Errorf("getting buffer for append: %w", err)
	} else {
		// Append to existing buffer
		buffer.Pending = append(buffer.Pending, aggregates...)
	}

	if err := sw.store.Put(ctx, aggBufferKey{}, buffer); err != nil {
		return fmt.Errorf("saving buffer after append: %w", err)
	}

	return nil
}

// ClearAggregates atomically clears the pending aggregates while preserving other state
func (sw *submissionWorkspace) ClearAggregates(ctx context.Context) error {
	sw.storeMu.Lock()
	defer sw.storeMu.Unlock()

	return sw.store.Put(ctx, aggBufferKey{}, Aggregates{
		Pending: []datamodel.Link{},
	})
}

// PutBuffer updates the submission buffer state
func (sw *submissionWorkspace) writeBuffer(ctx context.Context, buffer Aggregates) error {
	return sw.store.Put(ctx, aggBufferKey{}, buffer)
}
