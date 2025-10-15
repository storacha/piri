package aws_test

import (
	"bytes"
	"encoding/hex"
	"net/url"
	"os"
	"runtime"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/testutil"
	paws "github.com/storacha/piri/pkg/aws"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
)

func TestS3StoreReplace(t *testing.T) {
	if os.Getenv("CI") != "" || runtime.GOOS != "linux" {
		t.SkipNow()
	}

	endpoint := createS3(t)
	client := newS3Client(t, endpoint)

	bucketName := hex.EncodeToString(testutil.RandomBytes(t, 16))
	createBucket(t, client, bucketName)

	st := paws.NewS3StoreWithClient(client, bucketName, "")

	t.Run("conditional write", func(t *testing.T) {
		key := hex.EncodeToString(testutil.RandomBytes(t, 4))
		first := testutil.RandomBytes(t, 32)
		second := testutil.RandomBytes(t, 32)
		third := testutil.RandomBytes(t, 32)

		err := st.Put(t.Context(), key, 32, bytes.NewReader(first))
		require.NoError(t, err)

		err = st.Replace(t.Context(), key, bytes.NewReader(first), 32, bytes.NewReader(second))
		require.NoError(t, err)

		err = st.Replace(t.Context(), key, bytes.NewReader(first), 32, bytes.NewReader(third))
		require.ErrorIs(t, err, store.ErrPreconditionFailed)
	})

	t.Run("conditional first write", func(t *testing.T) {
		key := hex.EncodeToString(testutil.RandomBytes(t, 4))
		first := testutil.RandomBytes(t, 32)

		err := st.Replace(t.Context(), key, nil, 32, bytes.NewReader(first))
		require.NoError(t, err)
	})
}

func createS3(t *testing.T) *url.URL {
	container, err := minio.Run(t.Context(), "minio/minio:latest")
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)

	addr, err := container.ConnectionString(t.Context())
	require.NoError(t, err)

	return testutil.Must(url.Parse("http://" + addr))(t)
}

func newS3Client(t *testing.T, endpoint *url.URL) *s3.Client {
	cfg, err := config.LoadDefaultConfig(
		t.Context(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "minioadmin",
			},
		}),
		func(o *config.LoadOptions) error {
			o.Region = "us-east-1"
			return nil
		},
	)
	require.NoError(t, err)

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		base := endpoint.String()
		o.BaseEndpoint = &base
		o.UsePathStyle = true
	})
}

func createBucket(t *testing.T, client *s3.Client, name string) {
	_, err := client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(name)})
	require.NoError(t, err)
}
