package receiptstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-ucanto/core/car"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/receipt"
	rdm "github.com/storacha/go-ucanto/core/receipt/datamodel"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/genericstore"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/dsadapter"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

// ReceiptStore stores UCAN invocation receipts.
type ReceiptStore interface {
	// Get retrieves a receipt by its CID.
	Get(context.Context, ucan.Link) (receipt.AnyReceipt, error)
	// GetByRan retrieves a receipt by "ran" CID.
	GetByRan(context.Context, ucan.Link) (receipt.AnyReceipt, error)
	// Put adds or replaces a receipt in the store.
	Put(context.Context, receipt.AnyReceipt) error
}

// RanLinkIndex maps "ran" links to receipt root links.
type RanLinkIndex interface {
	Put(ctx context.Context, ran datamodel.Link, lnk datamodel.Link) error
	Get(ctx context.Context, ran datamodel.Link) (datamodel.Link, error)
}

// KeyEncoder defines how to encode keys for a specific backend.
type KeyEncoder interface {
	EncodeKey(link ucan.Link) string
}

// Store implements ReceiptStore backed by genericstore.
type Store struct {
	store        *genericstore.Store[receipt.AnyReceipt]
	ranLinkIndex RanLinkIndex
	encoder      KeyEncoder
}

var _ ReceiptStore = (*Store)(nil)

// New creates a ReceiptStore with the given backend, prefix, key encoder, and ran link index.
func New(backend objectstore.ListableStore, prefix string, encoder KeyEncoder, ranLinkIndex RanLinkIndex) *Store {
	return &Store{
		store:        genericstore.New[receipt.AnyReceipt](backend, prefix, Codec{}),
		ranLinkIndex: ranLinkIndex,
		encoder:      encoder,
	}
}

func (s *Store) Get(ctx context.Context, link ucan.Link) (receipt.AnyReceipt, error) {
	rcpt, err := s.store.Get(ctx, s.encoder.EncodeKey(link))
	if err != nil {
		return nil, fmt.Errorf("getting receipt: %w", err)
	}
	return rcpt, nil
}

func (s *Store) GetByRan(ctx context.Context, ran ucan.Link) (receipt.AnyReceipt, error) {
	root, err := s.ranLinkIndex.Get(ctx, ran)
	if err != nil {
		return nil, fmt.Errorf("looking up root by ran: %w", err)
	}
	// Convert datamodel.Link to ucan.Link for the key encoder
	rootLink, ok := root.(ucan.Link)
	if !ok {
		// Handle cidlink.Link case
		if cl, ok := root.(cidlink.Link); ok {
			rootLink = cl
		} else {
			return nil, fmt.Errorf("unexpected link type: %T", root)
		}
	}
	rcpt, err := s.store.Get(ctx, s.encoder.EncodeKey(rootLink))
	if err != nil {
		return nil, fmt.Errorf("getting receipt: %w", err)
	}
	return rcpt, nil
}

func (s *Store) Put(ctx context.Context, rcpt receipt.AnyReceipt) error {
	err := s.store.Put(ctx, s.encoder.EncodeKey(rcpt.Root().Link()), rcpt)
	if err != nil {
		return fmt.Errorf("storing receipt: %w", err)
	}
	err = s.ranLinkIndex.Put(ctx, rcpt.Ran().Link(), rcpt.Root().Link())
	if err != nil {
		return fmt.Errorf("indexing receipt by ran: %w", err)
	}
	return nil
}

// Codec implements genericstore.Codec for receipt.AnyReceipt.
type Codec struct{}

func (Codec) Encode(rcpt receipt.AnyReceipt) ([]byte, error) {
	r := car.Encode([]datamodel.Link{rcpt.Root().Link()}, rcpt.Blocks())
	return io.ReadAll(r)
}

func (Codec) Decode(data []byte) (receipt.AnyReceipt, error) {
	roots, blocks, err := car.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding car: %w", err)
	}
	br, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(blocks))
	if err != nil {
		return nil, fmt.Errorf("creating block reader: %w", err)
	}
	rcpt, err := receipt.NewReceipt[datamodel.Node, datamodel.Node](roots[0], br, rdm.TypeSystem().TypeByName("Receipt"))
	if err != nil {
		return nil, fmt.Errorf("decoding receipt: %w", err)
	}
	return rcpt, nil
}

// S3KeyEncoder encodes keys for S3/MinIO backends.
type S3KeyEncoder struct{}

func (S3KeyEncoder) EncodeKey(link ucan.Link) string {
	return link.String()
}

// DatastoreKeyEncoder encodes keys for LevelDB/datastore backends.
type DatastoreKeyEncoder struct{}

func (DatastoreKeyEncoder) EncodeKey(link ucan.Link) string {
	return link.String()
}

// S3RanLinkIndex implements RanLinkIndex using S3/MinIO storage.
type S3RanLinkIndex struct {
	store  *minio.Store
	prefix string
}

func (idx *S3RanLinkIndex) Put(ctx context.Context, ran datamodel.Link, lnk datamodel.Link) error {
	key := idx.prefix + ran.String() + ".ref"
	cidStr := lnk.String()
	return idx.store.Put(ctx, key, uint64(len(cidStr)), strings.NewReader(cidStr))
}

func (idx *S3RanLinkIndex) Get(ctx context.Context, ran datamodel.Link) (datamodel.Link, error) {
	key := idx.prefix + ran.String() + ".ref"
	obj, err := idx.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	defer obj.Body().Close()
	data, err := io.ReadAll(obj.Body())
	if err != nil {
		return nil, err
	}
	c, err := cid.Parse(string(data))
	if err != nil {
		return nil, err
	}
	return cidlink.Link{Cid: c}, nil
}

// DatastoreRanLinkIndex implements RanLinkIndex using datastore.
type DatastoreRanLinkIndex struct {
	ds datastore.Datastore
}

func (idx *DatastoreRanLinkIndex) Put(ctx context.Context, ran datamodel.Link, lnk datamodel.Link) error {
	return idx.ds.Put(ctx, datastore.NewKey(ran.String()), []byte(lnk.Binary()))
}

func (idx *DatastoreRanLinkIndex) Get(ctx context.Context, ran datamodel.Link) (datamodel.Link, error) {
	data, err := idx.ds.Get(ctx, datastore.NewKey(ran.String()))
	if err != nil {
		return nil, err
	}
	c, err := cid.Cast(data)
	if err != nil {
		return nil, err
	}
	return cidlink.Link{Cid: c}, nil
}

// NewS3Store creates a ReceiptStore for S3/MinIO backends.
// Receipts are stored with prefix "receipts/" and ran index with "receipts-ran/".
func NewS3Store(backend *minio.Store) *Store {
	return New(
		backend,
		"receipts/",
		S3KeyEncoder{},
		&S3RanLinkIndex{store: backend, prefix: "receipts-ran/"},
	)
}

// NewDatastoreStore creates a ReceiptStore for LevelDB/datastore backends.
func NewDatastoreStore(ds datastore.Datastore) *Store {
	receiptsDs := namespace.Wrap(ds, datastore.NewKey("receipts/"))
	ranIndexDs := namespace.Wrap(ds, datastore.NewKey("ranLinkIndex/"))
	return New(
		dsadapter.New(receiptsDs),
		"",
		DatastoreKeyEncoder{},
		&DatastoreRanLinkIndex{ds: ranIndexDs},
	)
}
