package verifyread

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyRead(t *testing.T) {
	// Helper to create test data with known hash
	createTestData := func(content string) (data []byte, hash []byte) {
		data = []byte(content)
		h := sha256.Sum256(data)
		return data, h[:]
	}

	t.Run("successful validation", func(t *testing.T) {
		// Create test data
		data, expectedHash := createTestData("Hello, World!")
		source := bytes.NewReader(data)

		// Create validating reader
		reader := New(
			source,
			sha256.New(),
			expectedHash,
		)

		// Consumer reads all data
		result := &bytes.Buffer{}
		n, err := io.Copy(result, reader)

		// Should succeed
		assert.NoError(t, err)
		assert.Equal(t, int64(len(data)), n)
		assert.Equal(t, data, result.Bytes())
		assert.Equal(t, uint64(len(data)), reader.BytesRead())
	})

	t.Run("hash mismatch causes consumer failure", func(t *testing.T) {
		// Create test data
		data, _ := createTestData("Hello, World!")
		source := bytes.NewReader(data)

		// Use wrong hash
		wrongHash := sha256.Sum256([]byte("Different content"))

		// Create validating reader with wrong hash
		reader := New(
			source,
			sha256.New(),
			wrongHash[:],
		)

		// Consumer tries to read all data
		result := &bytes.Buffer{}
		n, err := io.Copy(result, reader)

		// Should FAIL with hash validation error
		assert.ErrorIs(t, err, ErrHashMismatch)

		// Note: io.Copy may have written bytes before the error was detected at EOF.
		// This is expected behavior - validation happens after all data is read.
		// The important thing is that the operation returns an error, and the
		// consumer should discard/cleanup the partially written data.
		assert.Equal(t, int64(len(data)), n)  // Bytes were written before validation failed
		assert.Equal(t, data, result.Bytes()) // Data was copied but operation failed
	})

	t.Run("partial reads", func(t *testing.T) {
		// Test that validation happens even with small buffer reads
		data, expectedHash := createTestData("Hello, World! This is a longer message.")
		source := bytes.NewReader(data)

		reader := New(
			source,
			sha256.New(),
			expectedHash,
		)

		// Read in small chunks
		result := &bytes.Buffer{}
		buf := make([]byte, 4) // Small 4-byte buffer

		for {
			n, err := reader.Read(buf)
			if n > 0 {
				result.Write(buf[:n])
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		assert.Equal(t, data, result.Bytes())
		assert.True(t, reader.Validated())
	})

	t.Run("chaining", func(t *testing.T) {
		data := bytes.Repeat([]byte("a"), 10*1024)

		shaDigest := sha256.Sum256(data)
		md5Digest := md5.Sum(data)

		reader0 := bytes.NewReader(data)

		reader1 := New(
			reader0,
			sha256.New(),
			shaDigest[:],
		)

		reader2 := New(
			reader1,
			md5.New(),
			md5Digest[:])

		// Consumer reads all data
		result := &bytes.Buffer{}
		n, err := io.Copy(result, reader2)

		// Should succeed
		assert.NoError(t, err)
		assert.Equal(t, int64(len(data)), n)
		assert.Equal(t, data, result.Bytes())
		assert.Equal(t, uint64(len(data)), reader1.BytesRead())
		assert.Equal(t, uint64(len(data)), reader2.BytesRead())

	})
}

func BenchmarkHashValidatingReader(b *testing.B) {
	// Create 10MB of test data
	data := bytes.Repeat([]byte("a"), 10*1024*1024)
	hash := sha256.Sum256(data)

	b.Run("WithValidation", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			reader := New(

				bytes.NewReader(data),
				sha256.New(),
				hash[:],
			)
			io.Copy(io.Discard, reader)
		}
	})

	b.Run("WithoutValidation", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			io.Copy(io.Discard, reader)
		}
	})
}
