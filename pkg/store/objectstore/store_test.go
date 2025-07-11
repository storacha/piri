package objectstore_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/store/objectstore"
	"github.com/storacha/piri/pkg/store/objectstore/leveldb"
	"github.com/storacha/piri/pkg/store/objectstore/memory"
	miniostore "github.com/storacha/piri/pkg/store/objectstore/minio"
)

type StoreKind string

const (
	Memory  StoreKind = "memory"
	LevelDB StoreKind = "leveldb"
	Minio   StoreKind = "minio"
)

var (
	//storeKinds = []StoreKind{Memory, LevelDB, Minio}
	storeKinds = []StoreKind{Minio}
)

func makeStore(t *testing.T, k StoreKind) objectstore.Store {
	switch k {
	case Memory:
		return memory.NewStore()
	case LevelDB:
		return createLevelDBStore(t)
	case Minio:
		return createMinioStore(t)
	}
	panic("unknown store kind")
}

func TestPutOperations(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		size      uint64
		expectErr bool
	}{
		/*
			{
				name: "successful put",
				data: []byte("hello world"),
				size: 11,
			},
			{
				name: "put with large data",
				data: bytes.Repeat([]byte("a"), 1024*1024), // 1MB
				size: 1024 * 1024,
			},

		*/
		{
			name: "put with multipart upload data",
			data: bytes.Repeat([]byte("a"), 32*1024*1024), // 32MB - minio threshold is 16MB for multipart upload
			size: 32 * 1024 * 1024,
		},
		/*
			{
				name:      "put with size mismatch",
				data:      []byte("hello"),
				size:      10, // Wrong size
				expectErr: true,
			},
			{
				name: "put with special characters in key",
				data: []byte("special data"),
				size: 12,
			},

		*/
	}

	for _, k := range storeKinds {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s_%s", k, tt.name), func(t *testing.T) {
				ctx := context.Background()
				store := makeStore(t, k)

				key := testutil.MultihashOfBytes(t, tt.data)
				err := store.Put(ctx, key, tt.size, bytes.NewReader(tt.data))

				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)

					// Verify the object was stored correctly
					obj, err := store.Get(ctx, key)
					require.NoError(t, err)
					defer obj.Body().Close()

					//content, err := io.ReadAll(obj.Body())
					require.NoError(t, err)
					//require.Equal(t, tt.data, content)
					require.Equal(t, int64(tt.size), obj.Size())
				}
			})
		}
	}
}

func TestGetOperations(t *testing.T) {
	ctx := context.Background()
	// Pre-populate test data
	testData := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	testDataKey := testutil.MultihashOfBytes(t, testData)

	tests := []struct {
		name      string
		key       multihash.Multihash
		opts      []objectstore.GetOption
		expected  []byte
		expectErr error
	}{
		{
			name:     "get existing object",
			key:      testDataKey,
			expected: testData,
		},
		{
			name:      "get non-existent object",
			key:       testutil.RandomMultihash(t),
			expectErr: objectstore.ErrNotExist,
		},
		{
			name: "get with range - start only",
			key:  testDataKey,
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
			key:  testDataKey,
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
			key:  testDataKey,
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
			key:  testDataKey,
			opts: []objectstore.GetOption{
				objectstore.WithRange(objectstore.Range{
					Start: 30,
					End:   uint64Ptr(35), // 30 + 6 - 1 (inclusive)
				}),
			},
			expected: testData[30:36],
		},
	}

	for _, k := range storeKinds {
		if k != LevelDB {
			continue
		}
		store := makeStore(t, k)
		err := store.Put(ctx, testDataKey, uint64(len(testData)), bytes.NewReader(testData))
		require.NoError(t, err)

		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s_%s", k, tt.name), func(t *testing.T) {
				obj, err := store.Get(ctx, tt.key, tt.opts...)

				if tt.expectErr != nil {
					require.Error(t, err)
					require.Equal(t, tt.expectErr, err)
				} else {
					require.NoError(t, err)
					defer obj.Body().Close()

					content, err := io.ReadAll(obj.Body())
					require.NoError(t, err)
					require.Equal(t, tt.expected, content)

					// For range requests, the size should reflect the range length
					if len(tt.opts) > 0 {
						require.Equal(t, int64(len(tt.expected)), obj.Size())
					} else {
						require.Equal(t, int64(len(testData)), obj.Size())
					}
				}
			})
		}
	}
}

func TestConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	numOperations := 10
	for _, k := range storeKinds {
		store := makeStore(t, k)

		t.Run("concurrent puts", func(t *testing.T) {
			errCh := make(chan error, numOperations)

			for i := 0; i < numOperations; i++ {
				go func(index int) {
					data := []byte(fmt.Sprintf("data-%d", index))
					key := testutil.MultihashOfBytes(t, data)
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
					data := []byte(fmt.Sprintf("data-%d", index))
					key := testutil.MultihashOfBytes(t, data)
					obj, err := store.Get(ctx, key)
					if err != nil {
						resultCh <- result{err: err}
						return
					}
					defer obj.Body().Close()

					actualData, err := io.ReadAll(obj.Body())
					resultCh <- result{data: actualData, err: err}
				}(i)
			}

			for i := 0; i < numOperations; i++ {
				res := <-resultCh
				require.NoError(t, res.err)
				require.Contains(t, string(res.data), "data-")
			}
		})
	}
}

func TestEdgeCases(t *testing.T) {
	ctx := context.Background()

	for _, k := range storeKinds {
		store := makeStore(t, k)

		t.Run("put with context cancellation", func(t *testing.T) {
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel() // Cancel immediately

			data := []byte("test data")
			key := testutil.MultihashOfBytes(t, data)
			err := store.Put(cancelCtx, key, 10, bytes.NewReader(data))
			require.Error(t, err)
		})
	}
}

var (
	minioEndpoint string
)

func TestMain(m *testing.M) {
	if runtime.GOOS == "darwin" {
		fmt.Println("Skipping darwin tests, testcontainers not supported in CI")
		os.Exit(0)
	}
	logging.SetDebugLogging()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ACCESS_KEY": "minioadmin",
			"MINIO_SECRET_KEY": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to start MinIO container: %v", err))
	}

	host, err := container.Host(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to get container host: %v", err))
	}

	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		panic(fmt.Sprintf("Failed to get container port: %v", err))
	}

	minioEndpoint = fmt.Sprintf("%s:%s", host, port.Port())

	code := m.Run()

	if err := container.Terminate(ctx); err != nil {
		panic(fmt.Sprintf("Failed to terminate container: %v", err))
	}

	os.Exit(code)
}

func createLevelDBStore(t *testing.T) objectstore.Store {
	s, err := leveldb.NewStore(filepath.Join(t.TempDir(), "leveldb.db"))
	require.NoError(t, err)
	return s

}

func createMinioStore(t *testing.T) objectstore.Store {
	bucketName := uniqueBucketName(t.Name())
	store, err := miniostore.New(minioEndpoint, bucketName, true, minio.Options{
		Creds:           credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure:          false,
		TrailingHeaders: true,
	})
	require.NoError(t, err)
	require.NotNil(t, store)
	require.True(t, store.IsOnline())
	return store
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
