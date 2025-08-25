package blobstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/multiformats/go-multihash"
)

// BlobCleanupService provides cleanup functionality for blobstore implementations
type BlobCleanupService struct {
	blobstore Blobstore
	basePath  string // For filesystem-based blobstore
}

// NewBlobCleanupService creates a new blob cleanup service
func NewBlobCleanupService(bs Blobstore, basePath string) *BlobCleanupService {
	return &BlobCleanupService{
		blobstore: bs,
		basePath:  basePath,
	}
}

// Delete removes a blob from the blobstore
func (b *BlobCleanupService) Delete(ctx context.Context, digest multihash.Multihash) error {
	// Try to get the blob first to verify it exists
	_, err := b.blobstore.Get(ctx, digest)
	if err != nil {
		return fmt.Errorf("blob not found: %w", err)
	}

	// For filesystem-based blobstore, delete the file
	if err := b.deleteFromFilesystem(digest); err != nil {
		return fmt.Errorf("deleting blob from filesystem: %w", err)
	}

	return nil
}

// deleteFromFilesystem removes a blob file from the filesystem
func (b *BlobCleanupService) deleteFromFilesystem(digest multihash.Multihash) error {
	// Convert multihash to file path
	// This assumes the filesystem blobstore uses the multihash as the filename
	// You may need to adjust this based on your actual blobstore implementation
	filename := digest.B58String()
	filePath := filepath.Join(b.basePath, filename)

	// Remove the file
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, which is fine
			return nil
		}
		return fmt.Errorf("removing blob file: %w", err)
	}

	return nil
}

// GetBlobstore returns the underlying blobstore
func (b *BlobCleanupService) GetBlobstore() Blobstore {
	return b.blobstore
}

// Implement Blobstore interface by delegating to the underlying blobstore
func (b *BlobCleanupService) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	return b.blobstore.Put(ctx, digest, size, body)
}

func (b *BlobCleanupService) Get(ctx context.Context, digest multihash.Multihash, opts ...GetOption) (Object, error) {
	return b.blobstore.Get(ctx, digest, opts...)
}

// Implement FileSystemer interface if the underlying blobstore supports it
func (b *BlobCleanupService) FileSystem() http.FileSystem {
	if fs, ok := b.blobstore.(FileSystemer); ok {
		return fs.FileSystem()
	}
	return nil
}
