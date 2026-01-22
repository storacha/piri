package flatfs_test

import (
	"io"
	"strings"
	"testing"

	"github.com/storacha/piri/pkg/store/objectstore/flatfs"
	"github.com/storacha/piri/pkg/store/objectstore/memory"
	"github.com/stretchr/testify/require"
)

func TestFlatFSKeyAdapterStore(t *testing.T) {
	fun := flatfs.NextToLast(2)
	memStore := memory.NewStore()
	flatfsMemStore, err := flatfs.NewFlatFSKeyAdapter(t.Context(), memStore, fun)
	require.NoError(t, err)

	key := "ciqjkmoqchcaod3rhy26uo57r2ktpxq4lnmhye6njse7l2rkhoetrjy"
	value := "hello world"

	err = flatfsMemStore.Put(t.Context(), key, uint64(len(value)), strings.NewReader(value))
	require.NoError(t, err)

	// ensure SHARDING file was put
	_, err = memStore.Get(t.Context(), flatfs.SHARDING_FN)
	require.NoError(t, err)

	obj, err := flatfsMemStore.Get(t.Context(), key)
	require.NoError(t, err)
	body := obj.Body()

	b, err := io.ReadAll(body)
	require.NoError(t, err)
	body.Close()
	require.Equal(t, value, string(b))

	// check it was stored at the right path in the underlying store
	_, err = memStore.Get(t.Context(), "rj/"+key+".data")
	require.NoError(t, err)

	// ensure can create from existing
	_, err = flatfs.NewFlatFSKeyAdapter(t.Context(), memStore, fun)
	require.NoError(t, err)

	// ensure error when sharding function does not match
	_, err = flatfs.NewFlatFSKeyAdapter(t.Context(), memStore, flatfs.NextToLast(3))
	require.Error(t, err)
	require.ErrorContains(t, err, "sharding function mismatch")
}
