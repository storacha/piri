package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/storacha/piri/pkg/store/objectstore"
)

func TestPutOperations(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		data      []byte
		size      uint64
		expectErr bool
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			bucketName := uniqueBucketName(t.Name())
			store := createTestStore(t, bucketName)

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
}

func TestGetOperations(t *testing.T) {
	ctx := context.Background()
	bucketName := uniqueBucketName(t.Name())
	store := createTestStore(t, bucketName)

	// Pre-populate test data
	testData := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	err := store.Put(ctx, "test-object", uint64(len(testData)), bytes.NewReader(testData))
	require.NoError(t, err)

	tests := []struct {
		name      string
		key       string
		opts      []objectstore.GetOption
		expected  []byte
		expectErr bool
		errMsg    string
	}{
		{
			name:     "get existing object",
			key:      "test-object",
			expected: testData,
		},
		{
			name:      "get non-existent object",
			key:       "does-not-exist",
			expectErr: true,
			errMsg:    "get object with key",
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := store.Get(ctx, tt.key, tt.opts...)

			if tt.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
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

func TestConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	bucketName := uniqueBucketName(t.Name())
	store := createTestStore(t, bucketName)

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
}

func TestBucketCreation(t *testing.T) {
	t.Run("create new bucket", func(t *testing.T) {
		bucketName := uniqueBucketName(t.Name())
		store := createTestStore(t, bucketName)
		require.NotNil(t, store)
	})

	t.Run("use existing bucket", func(t *testing.T) {
		bucketName := uniqueBucketName(t.Name())

		// Create first store (creates bucket)
		store1 := createTestStore(t, bucketName)
		require.NotNil(t, store1)

		// Create second store (uses existing bucket)
		store2, err := New(minioEndpoint, bucketName, minio.Options{
			Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
			Secure: false,
		})
		require.NoError(t, err)
		require.NotNil(t, store2)
	})
}

func TestEdgeCases(t *testing.T) {
	ctx := context.Background()
	bucketName := uniqueBucketName(t.Name())
	store := createTestStore(t, bucketName)

	t.Run("put and get with unicode key", func(t *testing.T) {
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
}

var (
	minioEndpoint string
)

func TestMain(m *testing.M) {
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

func createTestStore(t *testing.T, bucketName string) *Store {
	store, err := New(minioEndpoint, bucketName, minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	require.NoError(t, err)
	require.NotNil(t, store)
	require.True(t, store.client.IsOnline())
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
