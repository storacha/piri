package aws

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
)

// S3Store implements the store.Store interface on S3
type S3Store struct {
	bucket    string
	keyPrefix string
	s3Client  *s3.Client
}

var _ store.Store = (*S3Store)(nil)

// Get implements store.Store.
func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	outPut, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.keyPrefix + key),
	})
	if err != nil {
		var noSuchKeyError *types.NoSuchKey
		// wrap in error recognizable as a not found error for Store interface consumers
		if errors.As(err, &noSuchKeyError) {
			return nil, store.NewErrNotFound(err)
		}
		return nil, err
	}
	return outPut.Body, nil
}

// Put implements store.Store.
func (s *S3Store) Put(ctx context.Context, key string, length uint64, data io.Reader) error {
	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.keyPrefix + key),
		Body:          data,
		ContentLength: aws.Int64(int64(length)),
	})
	return err
}

func (s *S3Store) Replace(ctx context.Context, key string, old io.Reader, length uint64, new io.Reader) error {
	input := s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.keyPrefix + key),
		Body:          new,
		ContentLength: aws.Int64(int64(length)),
	}

	// use conditional write requests to replace the data
	if old == nil {
		input.IfNoneMatch = aws.String("*")
	} else {
		b, err := io.ReadAll(old)
		if err != nil {
			return err
		}
		md5hash := md5.Sum(b)
		etag := fmt.Sprintf("%q", hex.EncodeToString(md5hash[:]))
		input.IfMatch = aws.String(etag)
	}

	_, err := s.s3Client.PutObject(ctx, &input)
	if err != nil {
		var oe smithy.APIError
		if errors.As(err, &oe) {
			// This method is used by the IPNI publisher to write a new head to the
			// chain. We can receive one of the following error code because we're
			// using `If-Match`.
			//
			// PreconditionFailed: At least one of the preconditions you specified did
			// not hold.
			//
			// OperationAborted: A conflicting conditional action is currently in
			// progress against this resource. Try again.
			//
			// If we receive OperationAborted then we'd get PreconditionFailed on the
			// next try, since we don't put the same head twice, and so there is no
			// chance to succeed with the same If-Match Etag, since the content will
			// have changed due to the "conflicting conditional action" already in
			// progress. Hence we don't try again and simply return the error
			// [store.ErrPreconditionFailed] so that the calling code can try again.
			//
			// When ErrPreconditionFailed is returned, a new advert must be
			// constructed that references the new head and the operation retried.
			if oe.ErrorCode() == "PreconditionFailed" || oe.ErrorCode() == "OperationAborted" {
				return store.ErrPreconditionFailed
			}
		}
	}
	return err
}

func NewS3StoreWithClient(client *s3.Client, bucket string, keyPrefix string) *S3Store {
	return &S3Store{s3Client: client, bucket: bucket, keyPrefix: keyPrefix}
}

func NewS3Store(cfg aws.Config, bucket string, keyPrefix string, opts ...func(*s3.Options)) *S3Store {
	return NewS3StoreWithClient(s3.NewFromConfig(cfg, opts...), bucket, keyPrefix)
}
