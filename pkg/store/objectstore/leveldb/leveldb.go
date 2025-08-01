package leveldb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/multiformats/go-multihash"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/storacha/piri/pkg/store/objectstore"
)

type leveldbStore struct {
	db *leveldb.DB
}

func NewStore(path string) (objectstore.Store, error) {
	opts := &opt.Options{
		NoSync: false,
	}
	db, err := leveldb.OpenFile(path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb: %w", err)
	}
	return &leveldbStore{db: db}, nil
}

func (s *leveldbStore) Put(ctx context.Context, key multihash.Multihash, size uint64, data io.Reader) error {
	buf := make([]byte, size)
	n, err := io.ReadFull(data, buf)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}
	if uint64(n) != size {
		return fmt.Errorf("expected %d bytes but read %d", size, n)
	}

	if err := s.db.Put(key, buf, nil); err != nil {
		return fmt.Errorf("failed to put data: %w", err)
	}

	return nil
}

func (s *leveldbStore) Get(ctx context.Context, key multihash.Multihash, opts ...objectstore.GetOption) (objectstore.Object, error) {
	data, err := s.db.Get(key, nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, objectstore.ErrNotExist
		}
		return nil, fmt.Errorf("failed to get data: %w", err)
	}

	cfg := objectstore.NewGetConfig()
	cfg.ProcessOptions(opts)
	r := cfg.Range()

	var start, end int64
	start = int64(r.Start)
	if r.End != nil {
		end = int64(*r.End) + 1
	} else {
		end = int64(len(data))
	}

	if start > int64(len(data)) {
		return nil, fmt.Errorf("range start %d exceeds data size %d", start, len(data))
	}
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	if start > end {
		return nil, fmt.Errorf("invalid range: start %d > end %d", start, end)
	}

	rangedData := data[start:end]
	return &leveldbObject{
		size: int64(len(rangedData)),
		body: io.NopCloser(bytes.NewReader(rangedData)),
	}, nil
}

func (s *leveldbStore) Close() error {
	return s.db.Close()
}

type leveldbObject struct {
	size int64
	body io.ReadCloser
}

func (o *leveldbObject) Size() int64 {
	return o.size
}

func (o *leveldbObject) Body() io.ReadCloser {
	return o.body
}
