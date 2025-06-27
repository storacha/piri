package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/storacha/piri/pkg/store/objectstore"
)

// Global variable to prevent compiler optimizations
var benchResult interface{}

func BenchmarkPut(b *testing.B) {
	ctx := context.Background()
	store := createBenchStore(b)

	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
	}

	for _, tc := range sizes {
		data := bytes.Repeat([]byte("a"), tc.size)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(tc.size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("bench-put-%s-%d", tc.name, i)
				err := store.Put(ctx, key, uint64(tc.size), bytes.NewReader(data))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGet(b *testing.B) {
	ctx := context.Background()
	store := createBenchStore(b)

	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
	}

	// Pre-populate data
	for _, tc := range sizes {
		data := bytes.Repeat([]byte("a"), tc.size)
		key := fmt.Sprintf("bench-get-%s", tc.name)
		err := store.Put(ctx, key, uint64(tc.size), bytes.NewReader(data))
		if err != nil {
			b.Fatal(err)
		}
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			key := fmt.Sprintf("bench-get-%s", tc.name)
			b.SetBytes(int64(tc.size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				obj, err := store.Get(ctx, key)
				if err != nil {
					b.Fatal(err)
				}

				// Read all data to ensure complete transfer
				data, err := io.ReadAll(obj.Body())
				if err != nil {
					b.Fatal(err)
				}
				obj.Body().Close()

				// Prevent compiler optimization
				benchResult = data
			}
		})
	}
}

func BenchmarkGetRange(b *testing.B) {
	ctx := context.Background()
	store := createBenchStore(b)

	// Create a 10MB object
	objectSize := 10 * 1024 * 1024
	data := bytes.Repeat([]byte("a"), objectSize)
	key := "bench-range-object"
	err := store.Put(ctx, key, uint64(objectSize), bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}

	rangeSizes := []struct {
		name  string
		start uint64
		end   uint64
	}{
		{"First1KB", 0, 1023},
		{"Middle1KB", 5 * 1024 * 1024, 5*1024*1024 + 1023},
		{"Last1KB", uint64(objectSize - 1024), uint64(objectSize - 1)},
		{"First1MB", 0, 1024*1024 - 1},
		{"Middle1MB", 5 * 1024 * 1024, 6*1024*1024 - 1},
	}

	for _, tc := range rangeSizes {
		b.Run(tc.name, func(b *testing.B) {
			rangeSize := tc.end - tc.start + 1
			b.SetBytes(int64(rangeSize))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				obj, err := store.Get(ctx, key, objectstore.WithRange(objectstore.Range{
					Start: tc.start,
					End:   &tc.end,
				}))
				if err != nil {
					b.Fatal(err)
				}

				// Read all data to ensure complete transfer
				data, err := io.ReadAll(obj.Body())
				if err != nil {
					b.Fatal(err)
				}
				obj.Body().Close()

				if len(data) != int(rangeSize) {
					b.Fatalf("expected %d bytes, got %d", rangeSize, len(data))
				}

				// Prevent compiler optimization
				benchResult = data
			}
		})
	}
}

func BenchmarkConcurrentPut(b *testing.B) {
	ctx := context.Background()
	store := createBenchStore(b)

	dataSize := 100 * 1024 // 100KB
	data := bytes.Repeat([]byte("a"), dataSize)

	concurrencyLevels := []int{1, 5, 10, 20}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", concurrency), func(b *testing.B) {
			b.SetBytes(int64(dataSize * concurrency))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				sem := make(chan struct{}, concurrency)
				errCh := make(chan error, concurrency)

				for j := 0; j < concurrency; j++ {
					sem <- struct{}{}
					go func(idx int) {
						defer func() { <-sem }()
						key := fmt.Sprintf("bench-concurrent-%d-%d-%d", concurrency, i, idx)
						err := store.Put(ctx, key, uint64(dataSize), bytes.NewReader(data))
						errCh <- err
					}(j)
				}

				// Wait for all goroutines
				for j := 0; j < concurrency; j++ {
					sem <- struct{}{}
				}
				close(errCh)

				// Check for errors
				for err := range errCh {
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

func BenchmarkConcurrentGet(b *testing.B) {
	ctx := context.Background()
	store := createBenchStore(b)

	dataSize := 100 * 1024 // 100KB
	data := bytes.Repeat([]byte("a"), dataSize)

	// Pre-populate objects
	numObjects := 20
	for i := 0; i < numObjects; i++ {
		key := fmt.Sprintf("bench-concurrent-get-%d", i)
		err := store.Put(ctx, key, uint64(dataSize), bytes.NewReader(data))
		if err != nil {
			b.Fatal(err)
		}
	}

	concurrencyLevels := []int{1, 5, 10, 20}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", concurrency), func(b *testing.B) {
			b.SetBytes(int64(dataSize * concurrency))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				sem := make(chan struct{}, concurrency)
				errCh := make(chan error, concurrency)

				for j := 0; j < concurrency; j++ {
					sem <- struct{}{}
					go func(idx int) {
						defer func() { <-sem }()
						key := fmt.Sprintf("bench-concurrent-get-%d", idx%numObjects)
						obj, err := store.Get(ctx, key)
						if err != nil {
							errCh <- err
							return
						}

						data, err := io.ReadAll(obj.Body())
						obj.Body().Close()
						if err != nil {
							errCh <- err
							return
						}

						// Prevent compiler optimization
						benchResult = data
						errCh <- nil
					}(j)
				}

				// Wait for all goroutines
				for j := 0; j < concurrency; j++ {
					sem <- struct{}{}
				}
				close(errCh)

				// Check for errors
				for err := range errCh {
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

func createBenchStore(b *testing.B) *Store {
	b.Helper()

	if minioEndpoint == "" {
		b.Skip("MinIO endpoint not available - run TestMain first")
	}

	bucketName := uniqueBucketName(b.Name())
	store, err := New(minioEndpoint, bucketName, minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		b.Fatal(err)
	}

	return store
}
