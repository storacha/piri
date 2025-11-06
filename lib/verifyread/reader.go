package verifyread

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
	"io"
)

var ErrHashMismatch = errors.New("hash validation failed")

type Reader struct {
	src         io.Reader
	h           hash.Hash
	expectedSum []byte

	bytesRead uint64
	done      bool  // reached EOF
	finalErr  error // latched terminal error (e.g., mismatch)
}

// If you pass nil values to `New` it will panic and you're an idiot.
func New(src io.Reader, h hash.Hash, expected []byte) *Reader {
	if src == nil {
		panic("source reader cannot be nil")
	}
	if h == nil {
		panic("source reader cannot be nil")
	}
	if len(expected) == 0 {
		panic("source reader cannot be nil")
	}
	return &Reader{src: src, h: h, expectedSum: expected}
}

func (r *Reader) Read(p []byte) (int, error) {
	if r.finalErr != nil {
		return 0, r.finalErr
	}
	if r.done {
		return 0, io.EOF
	}

	n, err := r.src.Read(p)
	if n > 0 {
		_, innErr := r.h.Write(p[:n])
		if innErr != nil {
			return 0, innErr
		}
		r.bytesRead += uint64(n)
	}

	if err == io.EOF {
		r.done = true
		sum := r.h.Sum(nil)
		if !bytes.Equal(sum, r.expectedSum) {
			r.finalErr = fmt.Errorf("%w: expected %x, got %x", ErrHashMismatch, r.expectedSum, sum)
			// return n (might be >0) + the error; caller sees last bytes and the failure
			return n, r.finalErr
		}
		return n, io.EOF
	}
	return n, err
}

func (r *Reader) BytesRead() uint64 { return r.bytesRead }

func (r *Reader) Validated() bool {
	return r.done
}
