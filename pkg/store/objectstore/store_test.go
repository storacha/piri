package objectstore_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/flatfs"
	"github.com/storacha/piri/pkg/store/objectstore/leveldb"
	"github.com/storacha/piri/pkg/store/objectstore/memory"
	miniostore "github.com/storacha/piri/pkg/store/objectstore/minio"
)

type StoreKind string

const (
	Memory  StoreKind = "memory"
	LevelDB StoreKind = "leveldb"
	Minio   StoreKind = "minio"
	FlatFS  StoreKind = "flatfs"
)

var (
	storeKinds = []StoreKind{Memory, LevelDB, Minio, FlatFS}
)

func makeStore(t *testing.T, k StoreKind) objectstore.Store {
	switch k {
	case Memory:
		return memory.NewStore()
	case LevelDB:
		return createLevelDBStore(t)
	case Minio:
		return createMinioStore(t)
	case FlatFS:
		return createFlatFSStore(t)
	}
	panic("unknown store kind")
}

func TestObjectStore(t *testing.T) {
	// logging.SetDebugLogging()

	for _, k := range storeKinds {
		t.Run(string(k), func(t *testing.T) {
			store := makeStore(t, k)

			t.Run("put operations", func(t *testing.T) {
				tests := []struct {
					name      string
					key       string
					data      []byte
					size      uint64
					expectErr bool
					skip      []StoreKind
				}{
					{
						name: "successful put",
						key:  "test-key",
						data: []byte("hello world"),
						size: 11,
					},
					{
						name: "put with empty data",
						key:  "empty-key",
						data: []byte{},
						size: 0,
					},
					{
						name: "put with large data",
						key:  "large-key",
						data: bytes.Repeat([]byte("a"), 1024*1024), // 1MB
						size: 1024 * 1024,
					},
					{
						name:      "put with size mismatch",
						key:       "mismatch-key",
						data:      []byte("hello"),
						size:      10, // Wrong size
						expectErr: true,
					},
					{
						name: "put with special characters in key",
						key:  "special/key/with-dashes_and.dots",
						data: []byte("special data"),
						size: 12,
						skip: []StoreKind{
							FlatFS, // no slashes allowed in flatfs
						},
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						if slices.Contains(tt.skip, k) {
							t.SkipNow()
						}
						ctx := t.Context()

						err := store.Put(ctx, tt.key, tt.size, bytes.NewReader(tt.data))

						if tt.expectErr {
							require.Error(t, err)
						} else {
							require.NoError(t, err)

							// Verify the object was stored correctly
							obj, err := store.Get(ctx, tt.key)
							require.NoError(t, err)
							defer obj.Body().Close()

							content, err := io.ReadAll(obj.Body())
							require.NoError(t, err)
							require.Equal(t, tt.data, content)
							require.Equal(t, int64(tt.size), obj.Size())
						}
					})
				}
			})

			t.Run("get operations", func(t *testing.T) {
				ctx := t.Context()
				// Pre-populate test data
				testData := []byte("0123456789abcdefghijklmnopqrstuvwxyz")

				tests := []struct {
					name      string
					key       string
					opts      []objectstore.GetOption
					expected  []byte
					expectErr error
				}{
					{
						name:     "get existing object",
						key:      "test-object",
						expected: testData,
					},
					{
						name:      "get non-existent object",
						key:       "does-not-exist",
						expectErr: objectstore.ErrNotExist,
					},
					{
						name: "get with range - start only",
						key:  "test-object",
						opts: []objectstore.GetOption{
							objectstore.WithRange(objectstore.Range{
								Start: 10,
								// End: nil means read to EOF
							}),
						},
						expected: testData[10:],
					},
					{
						name: "get with range - start and end",
						key:  "test-object",
						opts: []objectstore.GetOption{
							objectstore.WithRange(objectstore.Range{
								Start: 10,
								End:   uint64Ptr(19), // 10 + 10 - 1 (inclusive)
							}),
						},
						expected: testData[10:20],
					},
					{
						name: "get with range - from beginning",
						key:  "test-object",
						opts: []objectstore.GetOption{
							objectstore.WithRange(objectstore.Range{
								Start: 0,
								End:   uint64Ptr(4), // 0 + 5 - 1 (inclusive)
							}),
						},
						expected: testData[0:5],
					},
					{
						name: "get with range - near end",
						key:  "test-object",
						opts: []objectstore.GetOption{
							objectstore.WithRange(objectstore.Range{
								Start: 30,
								End:   uint64Ptr(35), // 30 + 6 - 1 (inclusive)
							}),
						},
						expected: testData[30:36],
					},
				}

				err := store.Put(ctx, "test-object", uint64(len(testData)), bytes.NewReader(testData))
				require.NoError(t, err)

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						obj, err := store.Get(ctx, tt.key, tt.opts...)

						if tt.expectErr != nil {
							require.Error(t, err)
							require.ErrorIs(t, err, tt.expectErr)
						} else {
							require.NoError(t, err)
							defer obj.Body().Close()

							content, err := io.ReadAll(obj.Body())
							require.NoError(t, err)
							require.Equal(t, tt.expected, content)
							require.Equal(t, int64(len(testData)), obj.Size())
						}
					})
				}
			})

			t.Run("concurrent operations", func(t *testing.T) {
				ctx := t.Context()
				numOperations := 10

				t.Run("concurrent puts", func(t *testing.T) {
					errCh := make(chan error, numOperations)

					for i := 0; i < numOperations; i++ {
						go func(index int) {
							key := fmt.Sprintf("concurrent-key-%d", index)
							data := []byte(fmt.Sprintf("data-%d", index))
							err := store.Put(ctx, key, uint64(len(data)), bytes.NewReader(data))
							errCh <- err
						}(i)
					}

					for i := 0; i < numOperations; i++ {
						require.NoError(t, <-errCh)
					}
				})

				t.Run("concurrent gets", func(t *testing.T) {
					type result struct {
						data []byte
						err  error
					}
					resultCh := make(chan result, numOperations)

					for i := 0; i < numOperations; i++ {
						go func(index int) {
							key := fmt.Sprintf("concurrent-key-%d", index)
							obj, err := store.Get(ctx, key)
							if err != nil {
								resultCh <- result{err: err}
								return
							}
							defer obj.Body().Close()

							data, err := io.ReadAll(obj.Body())
							resultCh <- result{data: data, err: err}
						}(i)
					}

					for i := 0; i < numOperations; i++ {
						res := <-resultCh
						require.NoError(t, res.err)
						require.Contains(t, string(res.data), "data-")
					}
				})
			})

			t.Run("edge cases", func(t *testing.T) {
				ctx := t.Context()

				t.Run("put and get with unicode key", func(t *testing.T) {
					if k == FlatFS {
						fmt.Println("unicode keys unsupported by FlatFS")
						t.SkipNow()
					}
					key := "unicode-key-测试-🚀"
					data := []byte("unicode content")
					err := store.Put(ctx, key, uint64(len(data)), bytes.NewReader(data))
					require.NoError(t, err)

					obj, err := store.Get(ctx, key)
					require.NoError(t, err)
					defer obj.Body().Close()

					content, err := io.ReadAll(obj.Body())
					require.NoError(t, err)
					require.Equal(t, data, content)
				})

				t.Run("put with context cancellation", func(t *testing.T) {
					cancelCtx, cancel := context.WithCancel(ctx)
					cancel() // Cancel immediately

					err := store.Put(cancelCtx, "cancelled-key", 10, bytes.NewReader([]byte("test data")))
					require.Error(t, err)
				})

				t.Run("put and delete", func(t *testing.T) {
					key := "test"
					data := []byte("test")
					err := store.Put(ctx, key, uint64(len(data)), bytes.NewReader(data))
					require.NoError(t, err)

					obj, err := store.Get(ctx, key)
					require.NoError(t, err)
					defer obj.Body().Close()

					content, err := io.ReadAll(obj.Body())
					require.NoError(t, err)
					require.Equal(t, data, content)

					err = store.Delete(ctx, key)
					require.NoError(t, err)

					_, err = store.Get(ctx, key)
					require.ErrorIs(t, err, objectstore.ErrNotExist)
				})
			})
		})
	}
}

func createLevelDBStore(t *testing.T) objectstore.Store {
	s, err := leveldb.NewStore(filepath.Join(t.TempDir(), "leveldb.db"))
	require.NoError(t, err)
	return s

}

func createMinioStore(t *testing.T) objectstore.Store {
	// This test expects docker to be running in linux CI environments and fails if it's not
	if testutil.IsRunningInCI(t) && runtime.GOOS == "linux" {
		if !testutil.IsDockerAvailable(t) {
			t.Fatalf("docker is expected in CI linux testing environments, but wasn't found")
		}
	}
	// otherwise this test is running locally, skip it if docker isn't available
	if !testutil.IsDockerAvailable(t) {
		t.SkipNow()
	}

	endpoint := testutil.StartMinioContainer(t)
	bucketName := uniqueBucketName(t.Name())
	store, err := miniostore.New(endpoint, bucketName, minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	require.NoError(t, err)
	require.NotNil(t, store)
	require.True(t, store.IsOnline())
	return store
}

func createFlatFSStore(t *testing.T) objectstore.Store {
	s, err := flatfs.New(filepath.Join(t.TempDir(), "flatfs"), flatfs.NextToLast(2), false)
	require.NoError(t, err)
	return s
}

func uniqueBucketName(testName string) string {
	// S3 bucket naming rules:
	// - Must be 3-63 characters
	// - Can only contain lowercase letters, numbers, and hyphens
	// - Cannot start or end with hyphen
	// - Cannot contain underscores or consecutive hyphens
	sanitized := strings.ToLower(testName)
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, " ", "-")

	// Remove any non-alphanumeric characters except hyphens
	var result []rune
	for _, r := range sanitized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result = append(result, r)
		}
	}
	sanitized = string(result)

	// Ensure no consecutive hyphens
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")

	// Create bucket name with timestamp
	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	bucketName := fmt.Sprintf("test-%s-%s", sanitized, ts[len(ts)-8:])

	// Ensure max 63 chars
	if len(bucketName) > 63 {
		// Keep last 8 chars of timestamp and adjust test name
		maxTestNameLen := 63 - 6 - 8 // "test-" (5) + "-" (1) + timestamp (8)
		if len(sanitized) > maxTestNameLen {
			sanitized = sanitized[:maxTestNameLen]
		}
		bucketName = fmt.Sprintf("test-%s-%s", sanitized, ts[len(ts)-8:])
	}

	return bucketName
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}
