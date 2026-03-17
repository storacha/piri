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
	backend   objectstore.ListableStore
	namespace string
	codec     Codec[T]
}

// Config holds configuration options for a generic store.
type Config struct {
	namespace string
}

// Option is a functional option for configuring a generic store.
type Option func(*Config)

// WithNamespace sets a key namespace that is prepended to all keys.
// This is useful when multiple logical stores share a single bucket.
func WithNamespace(namespace string) Option {
	return func(c *Config) {
		c.namespace = namespace
	}
}

// New creates a new generic store.
// By default, no namespace is used. Use WithNamespace to add a key namespace
// when multiple stores share the same backend.
func New[T any](backend objectstore.ListableStore, codec Codec[T], opts ...Option) *Store[T] {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Store[T]{
		backend:   backend,
		namespace: cfg.namespace,
		codec:     codec,
	}
}

// Get retrieves a value by its key.
func (s *Store[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T
	obj, err := s.backend.Get(ctx, s.namespace+key)
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

	err = s.backend.Put(ctx, s.namespace+key, uint64(len(data)), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("storing %s: %w", key, err)
	}

	return nil
}

// Delete removes a value by its key.
func (s *Store[T]) Delete(ctx context.Context, key string) error {
	return s.backend.Delete(ctx, s.namespace+key)
}

// Exists checks if a key exists (exact match).
func (s *Store[T]) Exists(ctx context.Context, key string) (bool, error) {
	return s.backend.Exists(ctx, s.namespace+key)
}

// ExistsWithPrefix checks if any key with the given prefix exists.
func (s *Store[T]) ExistsWithPrefix(ctx context.Context, keyPrefix string) (bool, error) {
	for _, err := range s.backend.ListPrefix(ctx, s.namespace+keyPrefix) {
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
	return s.GetAnyMatching(ctx, keyPrefix, nil)
}

// GetAnyMatching retrieves the first value matching the key prefix where the predicate returns true.
// If match is nil, returns the first value found (equivalent to GetAny).
// Returns store.ErrNotFound if no matching value exists.
func (s *Store[T]) GetAnyMatching(ctx context.Context, keyPrefix string, match func(T) bool) (T, error) {
	var zero T

	for item, err := range s.ListPrefix(ctx, keyPrefix) {
		if err != nil {
			return zero, err
		}
		if match == nil || match(item) {
			return item, nil
		}
	}

	return zero, store.ErrNotFound
}

// ListPrefix returns an iterator over all values with the given key prefix.
func (s *Store[T]) ListPrefix(ctx context.Context, keyPrefix string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T

		for key, err := range s.backend.ListPrefix(ctx, s.namespace+keyPrefix) {
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
