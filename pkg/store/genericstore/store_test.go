package genericstore

import (
	"encoding/json"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/objectstore/dsadapter"
)

// testItem is a simple struct for testing.
type testItem struct {
	ID    string
	Value int
}

// jsonCodec implements Codec using JSON encoding.
type jsonCodec struct{}

func (jsonCodec) Encode(item testItem) ([]byte, error) {
	return json.Marshal(item)
}

func (jsonCodec) Decode(data []byte) (testItem, error) {
	var item testItem
	err := json.Unmarshal(data, &item)
	return item, err
}

func TestGenericStore(t *testing.T) {
	testCases := []struct {
		name      string
		namespace string
	}{
		{name: "without namespace", namespace: ""},
		{name: "with namespace", namespace: "test/"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newStore := func() *Store[testItem] {
				backend := dsadapter.New(datastore.NewMapDatastore())
				if tc.namespace == "" {
					return New[testItem](backend, jsonCodec{})
				}
				return New[testItem](backend, jsonCodec{}, WithNamespace(tc.namespace))
			}

			t.Run("roundtrip", func(t *testing.T) {
				s := newStore()
				item := testItem{ID: "abc", Value: 42}

				err := s.Put(t.Context(), "key1", item)
				require.NoError(t, err)

				got, err := s.Get(t.Context(), "key1")
				require.NoError(t, err)
				require.Equal(t, item, got)
			})

			t.Run("delete", func(t *testing.T) {
				s := newStore()
				item := testItem{ID: "abc", Value: 42}

				err := s.Put(t.Context(), "key1", item)
				require.NoError(t, err)

				err = s.Delete(t.Context(), "key1")
				require.NoError(t, err)

				_, err = s.Get(t.Context(), "key1")
				require.ErrorIs(t, err, store.ErrNotFound)
			})

			t.Run("exists", func(t *testing.T) {
				s := newStore()

				exists, err := s.Exists(t.Context(), "key1")
				require.NoError(t, err)
				require.False(t, exists)

				err = s.Put(t.Context(), "key1", testItem{ID: "abc", Value: 42})
				require.NoError(t, err)

				exists, err = s.Exists(t.Context(), "key1")
				require.NoError(t, err)
				require.True(t, exists)
			})

			t.Run("not found", func(t *testing.T) {
				s := newStore()

				_, err := s.Get(t.Context(), "nonexistent")
				require.ErrorIs(t, err, store.ErrNotFound)
			})

			t.Run("exists with prefix", func(t *testing.T) {
				s := newStore()

				exists, err := s.ExistsWithPrefix(t.Context(), "group/")
				require.NoError(t, err)
				require.False(t, exists)

				err = s.Put(t.Context(), "group/item1", testItem{ID: "a", Value: 1})
				require.NoError(t, err)

				exists, err = s.ExistsWithPrefix(t.Context(), "group/")
				require.NoError(t, err)
				require.True(t, exists)

				exists, err = s.ExistsWithPrefix(t.Context(), "other/")
				require.NoError(t, err)
				require.False(t, exists)
			})

			t.Run("get any", func(t *testing.T) {
				s := newStore()
				item1 := testItem{ID: "a", Value: 1}
				item2 := testItem{ID: "b", Value: 2}

				err := s.Put(t.Context(), "group/item1", item1)
				require.NoError(t, err)
				err = s.Put(t.Context(), "group/item2", item2)
				require.NoError(t, err)

				got, err := s.GetAny(t.Context(), "group/")
				require.NoError(t, err)
				require.True(t, got == item1 || got == item2)
			})

			t.Run("get any not found", func(t *testing.T) {
				s := newStore()

				_, err := s.GetAny(t.Context(), "nonexistent/")
				require.ErrorIs(t, err, store.ErrNotFound)
			})

			t.Run("get any matching with predicate", func(t *testing.T) {
				s := newStore()
				item1 := testItem{ID: "a", Value: 10}
				item2 := testItem{ID: "b", Value: 20}
				item3 := testItem{ID: "c", Value: 30}

				err := s.Put(t.Context(), "group/item1", item1)
				require.NoError(t, err)
				err = s.Put(t.Context(), "group/item2", item2)
				require.NoError(t, err)
				err = s.Put(t.Context(), "group/item3", item3)
				require.NoError(t, err)

				// Find item with Value > 15
				got, err := s.GetAnyMatching(t.Context(), "group/", func(item testItem) bool {
					return item.Value > 15
				})
				require.NoError(t, err)
				require.True(t, got.Value > 15)
			})

			t.Run("get any matching no match", func(t *testing.T) {
				s := newStore()
				err := s.Put(t.Context(), "group/item1", testItem{ID: "a", Value: 10})
				require.NoError(t, err)

				// Predicate that never matches
				_, err = s.GetAnyMatching(t.Context(), "group/", func(item testItem) bool {
					return item.Value > 100
				})
				require.ErrorIs(t, err, store.ErrNotFound)
			})

			t.Run("list prefix", func(t *testing.T) {
				s := newStore()
				items := []testItem{
					{ID: "a", Value: 1},
					{ID: "b", Value: 2},
					{ID: "c", Value: 3},
				}

				for i, item := range items {
					err := s.Put(t.Context(), "group/item"+string(rune('0'+i)), item)
					require.NoError(t, err)
				}

				// Also add an item with different prefix
				err := s.Put(t.Context(), "other/item", testItem{ID: "x", Value: 99})
				require.NoError(t, err)

				var collected []testItem
				for item, err := range s.ListPrefix(t.Context(), "group/") {
					require.NoError(t, err)
					collected = append(collected, item)
				}

				require.Len(t, collected, 3)
				// Check all expected items are present
				ids := make(map[string]bool)
				for _, item := range collected {
					ids[item.ID] = true
				}
				require.True(t, ids["a"] && ids["b"] && ids["c"])
			})
		})
	}
}

func TestPrefixIsolation(t *testing.T) {
	backend := dsadapter.New(datastore.NewMapDatastore())
	store1 := New[testItem](backend, jsonCodec{}, WithNamespace("namespace1/"))
	store2 := New[testItem](backend, jsonCodec{}, WithNamespace("namespace2/"))

	item1 := testItem{ID: "a", Value: 1}
	item2 := testItem{ID: "b", Value: 2}

	err := store1.Put(t.Context(), "key", item1)
	require.NoError(t, err)
	err = store2.Put(t.Context(), "key", item2)
	require.NoError(t, err)

	got1, err := store1.Get(t.Context(), "key")
	require.NoError(t, err)
	require.Equal(t, item1, got1)

	got2, err := store2.Get(t.Context(), "key")
	require.NoError(t, err)
	require.Equal(t, item2, got2)

	// Verify isolation via ExistsWithPrefix
	exists, err := store1.ExistsWithPrefix(t.Context(), "")
	require.NoError(t, err)
	require.True(t, exists)

	// Delete from store1, store2 should be unaffected
	err = store1.Delete(t.Context(), "key")
	require.NoError(t, err)

	_, err = store1.Get(t.Context(), "key")
	require.ErrorIs(t, err, store.ErrNotFound)

	got2, err = store2.Get(t.Context(), "key")
	require.NoError(t, err)
	require.Equal(t, item2, got2)
}
