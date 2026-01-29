package receiptstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/minio"
)

const (
	minioReceiptsPrefix     = "receipts/"
	minioRanLinkIndexPrefix = "receipts-ran/"
)

// minioSimpleStore adapts a MinIO objectstore.Store to the SimpleStore interface
// required by the underlying ReceiptStore implementation.
type minioSimpleStore struct {
	store  *minio.Store
	prefix string
}

func (m *minioSimpleStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := m.store.Get(ctx, m.prefix+key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("getting %s from minio: %w", key, err)
	}
	return obj.Body(), nil
}

func (m *minioSimpleStore) Put(ctx context.Context, key string, size uint64, data io.Reader) error {
	return m.store.Put(ctx, m.prefix+key, size, data)
}

// minioRanLinkIndex implements RanLinkIndex using MinIO S3-compatible storage.
// It stores reference files that map ran links to receipt root links.
type minioRanLinkIndex struct {
	store  *minio.Store
	prefix string
}

func (m *minioRanLinkIndex) Put(ctx context.Context, ran datamodel.Link, lnk datamodel.Link) error {
	key := m.prefix + ran.String() + ".ref"
	cidStr := lnk.String()
	err := m.store.Put(ctx, key, uint64(len(cidStr)), strings.NewReader(cidStr))
	if err != nil {
		return fmt.Errorf("storing ran link index: %w", err)
	}
	return nil
}

func (m *minioRanLinkIndex) Get(ctx context.Context, ran datamodel.Link) (datamodel.Link, error) {
	key := m.prefix + ran.String() + ".ref"
	obj, err := m.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("getting ran link index: %w", err)
	}
	defer obj.Body().Close()

	data, err := io.ReadAll(obj.Body())
	if err != nil {
		return nil, fmt.Errorf("reading ran link index: %w", err)
	}

	c, err := cid.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing CID from ran link index: %w", err)
	}

	return cidlink.Link{Cid: c}, nil
}

// NewMinioReceiptStore creates a ReceiptStore backed by a MinIO S3-compatible store.
// Receipts are stored with keys prefixed by "receipts/" and the ran link index
// uses the prefix "receipts-ran/".
func NewMinioReceiptStore(s *minio.Store) (ReceiptStore, error) {
	simpleStore := &minioSimpleStore{
		store:  s,
		prefix: minioReceiptsPrefix,
	}
	ranIndex := &minioRanLinkIndex{
		store:  s,
		prefix: minioRanLinkIndexPrefix,
	}
	return NewReceiptStore(simpleStore, ranIndex)
}
