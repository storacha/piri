package genericstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/objectstore"
)

// Codec defines encoding/decoding for a value type.
type Codec[T any] interface {
	Encode(T) ([]byte, error)
	Decode([]byte) (T, error)
}

// Store is a generic key-value store backed by a ListableStore.
type Store[T any] struct {
	backend objectstore.ListableStore
	prefix  string
	codec   Codec[T]
}

// New creates a new generic store.
// The prefix is prepended to all keys when storing/retrieving from the backend.
func New[T any](backend objectstore.ListableStore, prefix string, codec Codec[T]) *Store[T] {
	return &Store[T]{
		backend: backend,
		prefix:  prefix,
		codec:   codec,
	}
}

// Get retrieves a value by its key.
func (s *Store[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T
	obj, err := s.backend.Get(ctx, s.prefix+key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotExist) {
			return zero, store.ErrNotFound
		}
		return zero, fmt.Errorf("getting %s: %w", key, err)
	}
	defer obj.Body().Close()

	data, err := io.ReadAll(obj.Body())
	if err != nil {
		return zero, fmt.Errorf("reading data: %w", err)
	}

	return s.codec.Decode(data)
}

// Put stores a value at the given key.
func (s *Store[T]) Put(ctx context.Context, key string, value T) error {
	data, err := s.codec.Encode(value)
	if err != nil {
		return fmt.Errorf("encoding value: %w", err)
	}

	err = s.backend.Put(ctx, s.prefix+key, uint64(len(data)), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("storing %s: %w", key, err)
	}

	return nil
}

// Delete removes a value by its key.
func (s *Store[T]) Delete(ctx context.Context, key string) error {
	return s.backend.Delete(ctx, s.prefix+key)
}

// Exists checks if a key exists (exact match).
func (s *Store[T]) Exists(ctx context.Context, key string) (bool, error) {
	return s.backend.Exists(ctx, s.prefix+key)
}

// ExistsWithPrefix checks if any key with the given prefix exists.
func (s *Store[T]) ExistsWithPrefix(ctx context.Context, keyPrefix string) (bool, error) {
	for _, err := range s.backend.ListPrefix(ctx, s.prefix+keyPrefix) {
		if err != nil {
			return false, err
		}
		// Found at least one key
		return true, nil
	}
	return false, nil
}

// GetAny retrieves any value matching the given key prefix.
// Returns store.ErrNotFound if no matching key exists.
func (s *Store[T]) GetAny(ctx context.Context, keyPrefix string) (T, error) {
	var zero T

	for key, err := range s.backend.ListPrefix(ctx, s.prefix+keyPrefix) {
		if err != nil {
			return zero, fmt.Errorf("listing prefix %s: %w", keyPrefix, err)
		}

		// Found a key, get the value
		obj, err := s.backend.Get(ctx, key)
		if err != nil {
			if errors.Is(err, objectstore.ErrNotExist) {
				// Key was deleted between list and get, continue
				continue
			}
			return zero, fmt.Errorf("getting %s: %w", key, err)
		}
		defer obj.Body().Close()

		data, err := io.ReadAll(obj.Body())
		if err != nil {
			return zero, fmt.Errorf("reading data: %w", err)
		}

		return s.codec.Decode(data)
	}

	return zero, store.ErrNotFound
}

// ListPrefix returns an iterator over all values with the given key prefix.
func (s *Store[T]) ListPrefix(ctx context.Context, keyPrefix string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T

		for key, err := range s.backend.ListPrefix(ctx, s.prefix+keyPrefix) {
			if err != nil {
				yield(zero, fmt.Errorf("listing prefix %s: %w", keyPrefix, err))
				return
			}

			obj, err := s.backend.Get(ctx, key)
			if err != nil {
				if errors.Is(err, objectstore.ErrNotExist) {
					continue
				}
				yield(zero, fmt.Errorf("getting %s: %w", key, err))
				return
			}

			data, err := io.ReadAll(obj.Body())
			obj.Body().Close()
			if err != nil {
				yield(zero, fmt.Errorf("reading data: %w", err))
				return
			}

			value, err := s.codec.Decode(data)
			if err != nil {
				yield(zero, fmt.Errorf("decoding data: %w", err))
				return
			}

			if !yield(value, nil) {
				return
			}
		}
	}
}
