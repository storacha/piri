package aggregator

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/piri/internal/ipldstore"
	"github.com/storacha/piri/pkg/pdp/aggregation/types"
)

type InProgressWorkspace interface {
	GetBuffer(context.Context) (types.Buffer, error)
	PutBuffer(context.Context, types.Buffer) error
}

type bufferKey struct{}

func (bufferKey) String() string { return "buffer" }

type inProgressWorkSpace struct {
	store ipldstore.KVStore[bufferKey, types.Buffer]
}

func (i *inProgressWorkSpace) GetBuffer(ctx context.Context) (types.Buffer, error) {
	buf, err := i.store.Get(ctx, bufferKey{})
	if store.IsNotFound(err) {
		err := i.store.Put(ctx, bufferKey{}, types.Buffer{})
		return types.Buffer{}, err
	}
	return buf, err
}

func (i *inProgressWorkSpace) PutBuffer(ctx context.Context, buffer types.Buffer) error {
	return i.store.Put(ctx, bufferKey{}, buffer)
}

const WorkspaceKey = "workspace/"

func newInProgressWorkspace(ds datastore.Datastore) InProgressWorkspace {
	ss := store.SimpleStoreFromDatastore(namespace.Wrap(ds, datastore.NewKey(WorkspaceKey)))
	return &inProgressWorkSpace{
		ipldstore.IPLDStore[bufferKey, types.Buffer](ss, types.BufferType(), captypes.Converters...),
	}
}
