package minio

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

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
