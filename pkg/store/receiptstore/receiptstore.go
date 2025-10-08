package receiptstore

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/core/car"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/receipt"
	rdm "github.com/storacha/go-ucanto/core/receipt/datamodel"
	"github.com/storacha/go-ucanto/ucan"
)

type RanLinkIndex interface {
	Put(ctx context.Context, ran datamodel.Link, lnk datamodel.Link) error
	Get(ctx context.Context, ran datamodel.Link) (datamodel.Link, error)
}

type receiptStore struct {
	store        store.SimpleStore
	ranLinkIndex RanLinkIndex
}

func (rs *receiptStore) Put(ctx context.Context, rcpt receipt.AnyReceipt) error {
	r := car.Encode([]datamodel.Link{rcpt.Root().Link()}, rcpt.Blocks())

	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("archiving delegation: %w", err)
	}

	err = rs.store.Put(ctx, rcpt.Root().Link().String(), uint64(len(b)), bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("writing to store: %w", err)
	}

	err = rs.ranLinkIndex.Put(ctx, rcpt.Ran().Link(), rcpt.Root().Link())
	if err != nil {
		return fmt.Errorf("indexing receipt by ran link: %w", err)
	}
	return nil
}

func (rs *receiptStore) getFromReader(r io.Reader) (receipt.AnyReceipt, error) {
	roots, blocks, err := car.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decoding car file: %w", err)
	}
	br, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(blocks))
	if err != nil {
		return nil, fmt.Errorf("setting up block reader: %w", err)
	}
	rcpt, err := receipt.NewReceipt[datamodel.Node, datamodel.Node](roots[0], br, rdm.TypeSystem().TypeByName("Receipt"))
	if err != nil {
		return nil, fmt.Errorf("decoding receipt: %w", err)
	}
	return rcpt, nil
}

func (rs *receiptStore) Get(ctx context.Context, root ucan.Link) (receipt.AnyReceipt, error) {

	r, err := rs.store.Get(ctx, root.String())
	if err != nil {
		return nil, fmt.Errorf("getting from store: %w", err)
	}
	defer r.Close()
	return rs.getFromReader(r)
}

func (rs *receiptStore) GetByRan(ctx context.Context, ran datamodel.Link) (receipt.AnyReceipt, error) {
	root, err := rs.ranLinkIndex.Get(ctx, ran)
	if err != nil {
		return nil, fmt.Errorf("looking up root: %w", err)
	}
	r, err := rs.store.Get(ctx, root.String())
	if err != nil {
		return nil, fmt.Errorf("getting from store: %w", err)
	}
	defer r.Close()
	return rs.getFromReader(r)
}

var _ ReceiptStore = (*receiptStore)(nil)

// NewDsDelegationStore creates a [DelegationStore] backed by an IPFS datastore.
func NewReceiptStore(store store.SimpleStore, ranLinkIndex RanLinkIndex) (ReceiptStore, error) {
	return &receiptStore{store, ranLinkIndex}, nil
}
