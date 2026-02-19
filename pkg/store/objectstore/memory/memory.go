package memory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/storacha/piri/pkg/store/objectstore"
)

type memoryStore struct {
	storeMu sync.RWMutex
	store   map[string][]byte
}

func NewStore() objectstore.Store {
	return &memoryStore{
		store: make(map[string][]byte),
	}
}

func (s *memoryStore) Delete(ctx context.Context, key string) error {
	s.storeMu.Lock()
	delete(s.store, key)
	s.storeMu.Unlock()
	return nil
}

func (s *memoryStore) Put(ctx context.Context, key string, size uint64, data io.Reader) error {
	buf := make([]byte, size)
	n, err := io.ReadFull(data, buf)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}
	if uint64(n) != size {
		return fmt.Errorf("expected %d bytes but read %d", size, n)
	}

	s.storeMu.Lock()
	s.store[key] = buf
	s.storeMu.Unlock()

	return nil
}

func (s *memoryStore) Get(ctx context.Context, key string, opts ...objectstore.GetOption) (objectstore.Object, error) {
	s.storeMu.RLock()
	data, exists := s.store[key]
	s.storeMu.RUnlock()

	if !exists {
		return nil, objectstore.ErrNotExist
	}

	cfg := objectstore.NewGetConfig()
	cfg.ProcessOptions(opts)
	r := cfg.Range()

	size := int64(len(data))
	start := int64(r.Start)

	// Only validate range if explicitly specified
	rangeSpecified := r.Start != 0 || r.End != nil
	if rangeSpecified {
		if start >= size {
			return nil, objectstore.ErrRangeNotSatisfiable{Range: r}
		}
		if r.End != nil && int64(*r.End) >= size {
			return nil, objectstore.ErrRangeNotSatisfiable{Range: r}
		}
	}

	var end int64
	if r.End != nil {
		end = int64(*r.End) + 1 // End is inclusive
	} else {
		end = size
	}

	rangedData := data[start:end]
	return &memoryObject{
		size: int64(len(rangedData)),
		body: io.NopCloser(bytes.NewReader(rangedData)),
	}, nil
}

type memoryObject struct {
	size int64
	body io.ReadCloser
}

func (o *memoryObject) Size() int64 {
	return o.size
}

func (o *memoryObject) Body() io.ReadCloser {
	return o.body
}
