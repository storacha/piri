package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
)

func StartMinioContainer(t *testing.T) string {
	container, err := minio.Run(t.Context(), "minio/minio:latest")
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)

	endpoint, err := container.ConnectionString(t.Context())
	require.NoError(t, err)

	t.Logf("Minio listening on: http://%s", endpoint)
	return endpoint
}
