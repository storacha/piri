package flatfs

import (
	"io"
	"os"
	"time"
)

// From: http://stackoverflow.com/questions/30697324/how-to-check-if-directory-on-path-is-empty
func DirIsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func openFile(filename string) (file *os.File, err error) {
	// Fallback retry for temporary error.
	for i := 0; i < RetryAttempts; i++ {
		file, err = os.Open(filename)
		if err == nil || !isTooManyFDError(err) {
			break
		}
		time.Sleep(time.Duration(i+1) * RetryDelay)
	}
	return
}

func tempFile(dir, pattern string) (fi *os.File, err error) {
	for i := 0; i < RetryAttempts; i++ {
		fi, err = os.CreateTemp(dir, pattern)
		if err == nil || !isTooManyFDError(err) {
			break
		}
		time.Sleep(time.Duration(i+1) * RetryDelay)
	}
	return fi, err
}

// rangeSatisfiable determines if the provided start/end byte range is
// valid given the total size of the blob.
func rangeSatisfiable(start uint64, end *uint64, size uint64) bool {
	if size > 0 && start >= size {
		return false
	}
	if end != nil {
		if start > *end {
			return false
		}
		if *end >= size {
			return false
		}
	}
	return true
}
