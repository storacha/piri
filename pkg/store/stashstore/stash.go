package stashstore

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// Stash provides methods for managing stashes within storage paths.
// Stashes are temporary files located in the "stash/" subdirectory of sealing paths.
// They are removed on startup and are not indexed. Stashes are used to store
// arbitrary data and can be served or removed as needed.
type Stash interface {
	// StashCreate creates a new stash file with the specified maximum size.
	// It selects a sealing path with the most available space and creates a file
	// named [uuid].tmp in the stash directory.
	//
	// The provided writeFunc is called with an *os.File pointing to the newly
	// created stash file, allowing the caller to write data into it.
	//
	// Parameters:
	//  - ctx: Context for cancellation and timeout.
	//  - maxSize: The maximum size of the stash file in bytes.
	//  - writeFunc: A function that writes data to the stash file.
	//
	// Returns:
	//  - uuid.UUID: A unique identifier for the created stash.
	//  - error: An error if the stash could not be created.
	StashCreate(ctx context.Context, maxSize int64, writeFunc func(f *os.File) error) (uuid.UUID, error)

	// StashRemove removes the stash file identified by the given UUID.
	//
	// Parameters:
	//  - ctx: Context for cancellation and timeout.
	//  - id: The UUID of the stash to remove.
	//
	// Returns:
	//  - error: An error if the stash could not be removed.
	StashRemove(ctx context.Context, id uuid.UUID) error

	// StashURL generates a URL for accessing the stash identified by the given UUID.
	//
	// Parameters:
	//  - id: The UUID of the stash.
	//
	// Returns:
	//  - url.URL: The URL where the stash can be accessed.
	//  - error: An error if the URL could not be generated.
	StashURL(id uuid.UUID) (url.URL, error)
}

type LocalStashStore struct {
	basePath string // absolute path to sealing path root
}

// NewStashStore creates a new stash store rooted at the given sealing path.
// The stash directory will be created if it doesn't exist.
func NewStashStore(basePath string) (*LocalStashStore, error) {
	stashPath := filepath.Join(basePath, "stash")
	if err := os.MkdirAll(stashPath, 0755); err != nil {
		return nil, fmt.Errorf("creating stash dir: %w", err)
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolving base path: %w", err)
	}
	return &LocalStashStore{basePath: absBase}, nil
}

func (s *LocalStashStore) stashFilePath(id uuid.UUID) string {
	return filepath.Join(s.basePath, "stash", fmt.Sprintf("%s.tmp", id.String()))
}

func (s *LocalStashStore) StashCreate(ctx context.Context, maxSize int64, writeFunc func(f *os.File) error) (uuid.UUID, error) {
	id := uuid.New()
	path := s.stashFilePath(id)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return uuid.Nil, fmt.Errorf("creating stash file: %w", err)
	}
	defer f.Close()

	if err := f.Truncate(maxSize); err != nil {
		return uuid.Nil, fmt.Errorf("truncating stash file: %w", err)
	}

	if err := writeFunc(f); err != nil {
		return uuid.Nil, fmt.Errorf("writeFunc failed: %w", err)
	}

	return id, nil
}

func (s *LocalStashStore) StashRemove(ctx context.Context, id uuid.UUID) error {
	path := s.stashFilePath(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing stash file: %w", err)
	}
	return nil
}

func (s *LocalStashStore) StashURL(id uuid.UUID) (url.URL, error) {
	path := s.stashFilePath(id)
	abs, err := filepath.Abs(path)
	if err != nil {
		return url.URL{}, fmt.Errorf("resolving absolute path: %w", err)
	}

	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(abs),
	}
	return u, nil
}

// OpenStashFromURL opens a file from a file:// URL and returns a ReadCloser.
func OpenStashFromURL(in string) (io.ReadCloser, error) {
	u, err := url.Parse(in)
	if err != nil {
		return nil, fmt.Errorf("parsing stash URL: %w", err)
	}
	if u.Scheme != "file" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	// Convert to native path format
	path := u.Path
	if filepath.Separator == '\\' && len(path) > 0 && path[0] == '/' && path[2] == ':' {
		// Windows: strip leading slash from "/C:/..."
		path = path[1:]
	}
	nativePath := filepath.FromSlash(path)

	return os.Open(nativePath)
}
