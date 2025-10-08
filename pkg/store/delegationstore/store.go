package delegationstore

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/ucan"
)

type delegationStore struct {
	data store.SimpleStore
}

func (d *delegationStore) Put(ctx context.Context, dlg delegation.Delegation) error {
	b, err := io.ReadAll(dlg.Archive())
	if err != nil {
		return fmt.Errorf("archiving delegation: %w", err)
	}
	err = d.data.Put(ctx, dlg.Link().String(), uint64(len(b)), bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("writing to datastore: %w", err)
	}
	return nil
}

func (d *delegationStore) Get(ctx context.Context, root ucan.Link) (delegation.Delegation, error) {

	r, err := d.data.Get(ctx, root.String())
	if err != nil {
		return nil, fmt.Errorf("getting from datastore: %w", err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading delegation data: %w", err)
	}
	dlg, err := delegation.Extract(data)
	if err != nil {
		return nil, fmt.Errorf("extracting delegation: %w", err)
	}

	return dlg, nil
}

// NewDelegationStore creates a [DelegationStore] backed by a simple store interface
func NewDelegationStore(store store.SimpleStore) (DelegationStore, error) {
	return &delegationStore{store}, nil
}
