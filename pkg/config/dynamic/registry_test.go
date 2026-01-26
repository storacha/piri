package dynamic

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/config"
)

// Test keys for registry tests
const (
	testKeyDuration config.Key = "test.duration"
	testKeyInt      config.Key = "test.int"
	testKeyUint     config.Key = "test.uint"
)

// mockPersister for testing persist/rollback behavior
type mockPersister struct {
	mu    sync.Mutex
	err   error
	calls []map[config.Key]any
}

func (m *mockPersister) Persist(updates map[config.Key]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, updates)
	return m.err
}

func TestNewRegistry(t *testing.T) {
	t.Run("creates registry with nil map", func(t *testing.T) {
		r := NewRegistry(nil)
		require.NotNil(t, r)
		require.Empty(t, r.Keys())
	})

	t.Run("creates registry with initial entries", func(t *testing.T) {
		entries := map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		}
		r := NewRegistry(entries)
		require.Len(t, r.Keys(), 1)
		require.Equal(t, 30*time.Second, r.GetDuration(testKeyDuration, 0))
	})

	t.Run("applies WithPersister option", func(t *testing.T) {
		p := &mockPersister{}
		r := NewRegistry(nil, WithPersister(p))
		require.NotNil(t, r.persister)
	})
}

func TestRegistry_RegisterEntries(t *testing.T) {
	t.Run("successfully registers new entries", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.RegisterEntries(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
			testKeyUint: {
				Value:  uint(10),
				Schema: UintSchema{Min: 1, Max: 100},
			},
		})
		require.NoError(t, err)
		require.Len(t, r.Keys(), 2)
	})

	t.Run("returns error on duplicate key", func(t *testing.T) {
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		})

		err := r.RegisterEntries(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  60 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "already registered")

		// Original value should be unchanged
		require.Equal(t, 30*time.Second, r.GetDuration(testKeyDuration, 0))
	})
}

func TestRegistry_Update(t *testing.T) {
	t.Run("successfully updates existing key", func(t *testing.T) {
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		})

		err := r.Update(map[string]any{
			string(testKeyDuration): "1m",
		}, false, SourceAPI)
		require.NoError(t, err)
		require.Equal(t, time.Minute, r.GetDuration(testKeyDuration, 0))
	})

	t.Run("returns UnknownKeyError for unknown key", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.Update(map[string]any{
			"unknown.key": "value",
		}, false, SourceAPI)
		require.Error(t, err)

		var unknownKeyErr *UnknownKeyError
		require.True(t, errors.As(err, &unknownKeyErr))
		require.Equal(t, "unknown.key", unknownKeyErr.Key)
	})

	t.Run("returns ValidationError when schema validation fails", func(t *testing.T) {
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Minute, Max: time.Hour},
			},
		})

		err := r.Update(map[string]any{
			string(testKeyDuration): "1s", // Below min of 1 minute
		}, false, SourceAPI)
		require.Error(t, err)

		var validationErr *ValidationError
		require.True(t, errors.As(err, &validationErr))
		require.Equal(t, testKeyDuration, validationErr.Key)
	})

	t.Run("notifies subscribers on change", func(t *testing.T) {
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		})

		var received ChangeEvent
		_, err := r.SubscribeFunc(testKeyDuration, func(e ChangeEvent) {
			received = e
		})
		require.NoError(t, err)

		err = r.Update(map[string]any{
			string(testKeyDuration): "1m",
		}, false, SourceAPI)
		require.NoError(t, err)

		require.Equal(t, testKeyDuration, received.Key)
		require.Equal(t, 30*time.Second, received.OldValue)
		require.Equal(t, time.Minute, received.NewValue)
		require.Equal(t, SourceAPI, received.Source)
	})

	t.Run("persists to file when persist=true", func(t *testing.T) {
		p := &mockPersister{}
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		}, WithPersister(p))

		err := r.Update(map[string]any{
			string(testKeyDuration): "1m",
		}, true, SourceAPI)
		require.NoError(t, err)

		require.Len(t, p.calls, 1)
		require.Equal(t, time.Minute, p.calls[0][testKeyDuration])
	})

	t.Run("does not persist when persist=false", func(t *testing.T) {
		p := &mockPersister{}
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		}, WithPersister(p))

		err := r.Update(map[string]any{
			string(testKeyDuration): "1m",
		}, false, SourceAPI)
		require.NoError(t, err)
		require.Empty(t, p.calls)
	})

	t.Run("rolls back in-memory changes when persist fails", func(t *testing.T) {
		p := &mockPersister{err: errors.New("disk full")}
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		}, WithPersister(p))

		err := r.Update(map[string]any{
			string(testKeyDuration): "1m",
		}, true, SourceAPI)
		require.Error(t, err)

		var persistErr *PersistError
		require.True(t, errors.As(err, &persistErr))

		// Value should be rolled back to original
		require.Equal(t, 30*time.Second, r.GetDuration(testKeyDuration, 0))
	})

	t.Run("empty updates map is no-op", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.Update(map[string]any{}, false, SourceAPI)
		require.NoError(t, err)
	})

	t.Run("multiple observers all notified", func(t *testing.T) {
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		})

		var count atomic.Int32
		for i := 0; i < 3; i++ {
			_, err := r.SubscribeFunc(testKeyDuration, func(e ChangeEvent) {
				count.Add(1)
			})
			require.NoError(t, err)
		}

		err := r.Update(map[string]any{
			string(testKeyDuration): "1m",
		}, false, SourceAPI)
		require.NoError(t, err)
		require.Equal(t, int32(3), count.Load())
	})
}

func TestRegistry_GetDuration(t *testing.T) {
	r := NewRegistry(map[config.Key]ConfigEntry{
		testKeyDuration: {
			Value:  30 * time.Second,
			Schema: DurationSchema{Min: time.Second, Max: time.Hour},
		},
		testKeyUint: {
			Value:  uint(10),
			Schema: UintSchema{Min: 1, Max: 100},
		},
	})

	t.Run("returns value when key exists", func(t *testing.T) {
		got := r.GetDuration(testKeyDuration, time.Hour)
		require.Equal(t, 30*time.Second, got)
	})

	t.Run("returns fallback when key doesn't exist", func(t *testing.T) {
		got := r.GetDuration("nonexistent", time.Hour)
		require.Equal(t, time.Hour, got)
	})

	t.Run("returns fallback when value is wrong type", func(t *testing.T) {
		got := r.GetDuration(testKeyUint, time.Hour)
		require.Equal(t, time.Hour, got)
	})
}

func TestRegistry_GetInt(t *testing.T) {
	r := NewRegistry(map[config.Key]ConfigEntry{
		testKeyInt: {
			Value:  50,
			Schema: IntSchema{Min: 0, Max: 100},
		},
		testKeyDuration: {
			Value:  30 * time.Second,
			Schema: DurationSchema{Min: time.Second, Max: time.Hour},
		},
	})

	t.Run("returns value when key exists", func(t *testing.T) {
		got := r.GetInt(testKeyInt, 0)
		require.Equal(t, 50, got)
	})

	t.Run("returns fallback when key doesn't exist", func(t *testing.T) {
		got := r.GetInt("nonexistent", 99)
		require.Equal(t, 99, got)
	})

	t.Run("returns fallback when value is wrong type", func(t *testing.T) {
		got := r.GetInt(testKeyDuration, 99)
		require.Equal(t, 99, got)
	})
}

func TestRegistry_GetUint(t *testing.T) {
	r := NewRegistry(map[config.Key]ConfigEntry{
		testKeyUint: {
			Value:  uint(50),
			Schema: UintSchema{Min: 0, Max: 100},
		},
		testKeyDuration: {
			Value:  30 * time.Second,
			Schema: DurationSchema{Min: time.Second, Max: time.Hour},
		},
	})

	t.Run("returns value when key exists", func(t *testing.T) {
		got := r.GetUint(testKeyUint, 0)
		require.Equal(t, uint(50), got)
	})

	t.Run("returns fallback when key doesn't exist", func(t *testing.T) {
		got := r.GetUint("nonexistent", 99)
		require.Equal(t, uint(99), got)
	})

	t.Run("returns fallback when value is wrong type", func(t *testing.T) {
		got := r.GetUint(testKeyDuration, 99)
		require.Equal(t, uint(99), got)
	})
}

func TestRegistry_GetAll(t *testing.T) {
	r := NewRegistry(map[config.Key]ConfigEntry{
		testKeyDuration: {
			Value:  30 * time.Second,
			Schema: DurationSchema{Min: time.Second, Max: time.Hour},
		},
		testKeyUint: {
			Value:  uint(50),
			Schema: UintSchema{Min: 0, Max: 100},
		},
	})

	all := r.GetAll()
	require.Len(t, all, 2)
	require.Equal(t, "30s", all[string(testKeyDuration)]) // Duration formatted as string
	require.Equal(t, uint(50), all[string(testKeyUint)])
}

func TestRegistry_Keys(t *testing.T) {
	r := NewRegistry(map[config.Key]ConfigEntry{
		testKeyDuration: {Value: 30 * time.Second, Schema: DurationSchema{}},
		testKeyUint:     {Value: uint(10), Schema: UintSchema{}},
	})

	keys := r.Keys()
	require.Len(t, keys, 2)
	require.Contains(t, keys, testKeyDuration)
	require.Contains(t, keys, testKeyUint)
}

func TestRegistry_Subscribe(t *testing.T) {
	t.Run("observer receives ChangeEvent on update", func(t *testing.T) {
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		})

		var received ChangeEvent
		_, err := r.Subscribe(testKeyDuration, ObserverFunc(func(e ChangeEvent) {
			received = e
		}))
		require.NoError(t, err)

		err = r.Update(map[string]any{string(testKeyDuration): "1m"}, false, SourceFile)
		require.NoError(t, err)

		require.Equal(t, testKeyDuration, received.Key)
		require.Equal(t, SourceFile, received.Source)
	})

	t.Run("unsubscribe function removes observer", func(t *testing.T) {
		r := NewRegistry(map[config.Key]ConfigEntry{
			testKeyDuration: {
				Value:  30 * time.Second,
				Schema: DurationSchema{Min: time.Second, Max: time.Hour},
			},
		})

		var count atomic.Int32
		unsubscribe, err := r.SubscribeFunc(testKeyDuration, func(e ChangeEvent) {
			count.Add(1)
		})
		require.NoError(t, err)

		// First update - should be received
		err = r.Update(map[string]any{string(testKeyDuration): "1m"}, false, SourceAPI)
		require.NoError(t, err)
		require.Equal(t, int32(1), count.Load())

		// Unsubscribe
		unsubscribe()

		// Second update - should NOT be received
		err = r.Update(map[string]any{string(testKeyDuration): "2m"}, false, SourceAPI)
		require.NoError(t, err)
		require.Equal(t, int32(1), count.Load()) // Still 1
	})

	t.Run("returns error when subscribing to non-existent key", func(t *testing.T) {
		r := NewRegistry(nil)
		_, err := r.Subscribe("nonexistent", ObserverFunc(func(e ChangeEvent) {}))
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not exist")
	})
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry(map[config.Key]ConfigEntry{
		testKeyDuration: {
			Value:  30 * time.Second,
			Schema: DurationSchema{Min: time.Second, Max: time.Hour},
		},
		testKeyUint: {
			Value:  uint(10),
			Schema: UintSchema{Min: 1, Max: 100},
		},
	})

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.GetDuration(testKeyDuration, 0)
			_ = r.GetUint(testKeyUint, 0)
			_ = r.Keys()
			_ = r.GetAll()
		}()
	}

	// Concurrent writes (with different valid values)
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			duration := time.Duration(i%60+1) * time.Second
			_ = r.Update(map[string]any{string(testKeyDuration): duration.String()}, false, SourceAPI)
		}(i)
	}

	// Concurrent subscribe/unsubscribe
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unsub, err := r.SubscribeFunc(testKeyDuration, func(e ChangeEvent) {})
			if err == nil {
				unsub()
			}
		}()
	}

	wg.Wait()
}
