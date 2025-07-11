package minio

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/minio/minio-go/v7"
	"github.com/multiformats/go-multihash"

	"github.com/storacha/piri/pkg/store/objectstore"
)

var log = logging.Logger("objectstore/minio")

type Store struct {
	client          *minio.Client
	bucket          string
	trailingHeaders bool
	// when set to true, the checksum of put operations will be compared with the
	// digest of the multihash key, if they match put is successful, if they don't, put fails and deletes the object.
	verifyOnPut bool
}

func New(endpoint, bucket string, verify bool, opts minio.Options) (*Store, error) {
	client, err := minio.New(endpoint, &opts)
	if err != nil {
		return nil, err
	}

	// allow for 5 seconds to check for existing bucket, and or create one.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if exists, err := client.BucketExists(ctx, bucket); err != nil {
		return nil, fmt.Errorf("failed to check if bucket %s exists: %s", bucket, err)
	} else if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket %s: %s", bucket, err)
		}
	}

	if verify && !opts.TrailingHeaders {
		return nil, fmt.Errorf("minio TrailingHeaders required for verify")
	}

	return &Store{
		client:          client,
		bucket:          bucket,
		trailingHeaders: opts.TrailingHeaders,
		verifyOnPut:     verify,
	}, nil
}

func (s *Store) IsOnline() bool {
	return s.client.IsOnline()
}

func (s *Store) Put(ctx context.Context, key multihash.Multihash, size uint64, body io.Reader) error {
	start := time.Now()
	log.Debugw("putting object", "bucket", s.bucket, "key", key, "size", size)

	putOpts := minio.PutObjectOptions{}
	// Only enable checksum if the client supports trailing headers
	if s.trailingHeaders {
		putOpts.Checksum = minio.ChecksumSHA256
	}

	obj, err := s.client.PutObject(
		ctx,
		s.bucket,
		key.String(),
		body,
		int64(size),
		putOpts,
	)
	if err != nil {
		log.Errorw("failed to put object", "bucket", s.bucket, "key", key, "size", size, "error", err)
		return fmt.Errorf("put object with key %s: %w", key, err)
	}

	// NB: it's highly unlikely this condition evaluates to true since minio will fail the Put operation
	// if the passed size doesn't match the `body` size. If for some reason that constrain isn't enforced for whatever
	// reason we can fall back to this.
	if obj.Size != int64(size) {
		log.Errorw("put object size mismatch", "bucket", s.bucket, "key", key, "expected_size", obj.Size, "actual_size", size)
		// Clean up the partial object
		deleteErr := s.client.RemoveObject(ctx, s.bucket, key.String(), minio.RemoveObjectOptions{})
		if deleteErr != nil {
			// Log but don't mask the original error
			log.Errorw("failed to clean up partial object", "bucket", s.bucket, "key", key, "error", deleteErr)
		}

		return fmt.Errorf("put object size mismatch: got %d, expected %d", obj.Size, size)
	}

	// Check if we need to validate the checksum
	if s.verifyOnPut {
		dm, err := multihash.Decode(key)
		if err != nil {
			return fmt.Errorf("failed to decode key to multihash for checksum verification: %w", err)
		}
		if dm.Code != multihash.SHA2_256 {
			codeStr := fmt.Sprintf("%d", dm.Code)
			if c, found := multihash.Codes[dm.Code]; found {
				codeStr = c
			}
			return fmt.Errorf("checksum verification failed for key multihash object. got %s, want %s ", codeStr, multihash.Codes[multihash.SHA2_256])
		}
		expectedChecksum := base64.StdEncoding.EncodeToString(dm.Digest)

		// The actual checksum is available in obj.ChecksumSHA256
		actualChecksum := obj.ChecksumSHA256

		// For multipart uploads, Minio returns checksums with a suffix like "-2" indicating the number of parts
		// We cannot verify the checksum in this case as the algorithm is different
		if strings.Contains(actualChecksum, "-") {
			log.Debugw("skipping checksum verification for multipart upload", "bucket", s.bucket, "key", key, "checksum", actualChecksum)
		} else if actualChecksum != expectedChecksum {
			log.Errorw("put object checksum mismatch", "bucket", s.bucket, "key", key, "expected_checksum", expectedChecksum, "actual_checksum", actualChecksum)
			// Clean up the object
			deleteErr := s.client.RemoveObject(ctx, s.bucket, key.String(), minio.RemoveObjectOptions{})
			if deleteErr != nil {
				// Log but don't mask the original error
				log.Errorw("failed to clean up object after checksum mismatch", "bucket", s.bucket, "key", key, "error", deleteErr)
			}

			return fmt.Errorf("put object checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
		}

		log.Debugw("put object checksum verified", "bucket", s.bucket, "key", key, "checksum", actualChecksum)
	}

	log.Debugw("put object", "bucket", s.bucket, "key", key, "size", size, "duration", time.Since(start), "checksum", obj.ChecksumSHA256)
	return nil
}

type MinioObject struct {
	object *minio.Object
	size   int64
}

func (o *MinioObject) Size() int64 {
	return o.size
}

func (o *MinioObject) Body() io.ReadCloser {
	return o.object
}

func (s *Store) Get(ctx context.Context, key multihash.Multihash, opts ...objectstore.GetOption) (objectstore.Object, error) {
	start := time.Now()
	config := objectstore.NewGetConfig()
	config.ProcessOptions(opts)
	log.Debugw("getting object", "bucket", s.bucket, "key", key, "options", config)

	miOpts := minio.GetObjectOptions{}
	// Check if a range is specified
	if config.Range().Start != 0 || config.Range().End != nil {
		rStart := int64(config.Range().Start)
		var rEnd int64

		if config.Range().End != nil {
			// Use the specified end position (inclusive)
			rEnd = int64(*config.Range().End)
		} else {
			// If no end specified, read to end of file
			// In minio-go, use 0 as end to read from start to EOF
			rEnd = 0
		}

		if err := miOpts.SetRange(rStart, rEnd); err != nil {
			log.Errorw("getting object failed to set range", "bucket", s.bucket, "key", key, "error", err)
			return nil, fmt.Errorf("invalid range options for key %s with start %d end %d: %w", key, rStart, rEnd, err)
		}
		log.Debugw("range set successfully", "start", rStart, "end", rEnd)
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key.String(), miOpts)
	if err != nil {
		log.Errorw("get object failed", "bucket", s.bucket, "key", key, "error", err)
		return nil, fmt.Errorf("get object with key %s: %w", key, err)
	}

	// For range requests, we cannot rely on Stat() due to a known issue in minio-go
	// where calling Stat() interferes with range requests and causes the entire file to be returned
	var size int64

	statObj, err := s.client.StatObject(ctx, s.bucket, key.String(), minio.StatObjectOptions{})
	if err != nil {
		var merr minio.ErrorResponse
		if errors.As(err, &merr) {
			if merr.Code == minio.NoSuchKey {
				return nil, objectstore.ErrNotExist
			}
		}
		log.Errorw("get object stat failed", "bucket", s.bucket, "key", key, "error", err)
		return nil, fmt.Errorf("get object with key %s: %w", key, err)
	}
	size = statObj.Size
	log.Debugw("got object", "bucket", s.bucket, "key", key, "size", size, "duration", time.Since(start), "options", config)

	if config.Range().Start != 0 || config.Range().End != nil {
		// For range requests, we cannot call Stat() as it breaks the range functionality, returning the entire object size
		// instead of the ranged-size.
		// Calculate the expected size based on the range parameters
		if config.Range().End != nil {
			// Size is end - start + 1 (since end is inclusive)
			size = int64(*config.Range().End - config.Range().Start + 1)
		} else {
			// For open-ended ranges (start to EOF), we need to get the full object size
			// We'll do a HEAD request separately to avoid interfering with the range request
			size = statObj.Size - int64(config.Range().Start)
		}
		log.Debugw("got object with range", "bucket", s.bucket, "key", key, "range_size", size, "duration", time.Since(start), "options", config)
	}

	return &MinioObject{
		object: obj,
		size:   size,
	}, nil
}
