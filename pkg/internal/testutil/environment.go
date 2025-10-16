package testutil

import (
	"net"
	"os"
	"testing"

	docker_client "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
)

// GetFreePort asks the OS for a free port that can be used for testing.
// This helps avoid port conflicts when running tests in parallel.
func GetFreePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "failed to get free port")

	port := listener.Addr().(*net.TCPAddr).Port

	// Close the listener so the port becomes available for the actual server
	require.NoError(t, listener.Close())

	return port
}

// IsRunningInCI returns true of process is running in CI environment.
func IsRunningInCI(t testing.TB) bool {
	t.Helper()
	return os.Getenv("CI") != ""
}

// IsDockerAvailable returns true if the docker daemon is available, useful for skipping tests when docker isn't running
func IsDockerAvailable(t testing.TB) bool {
	t.Helper()
	c, err := docker_client.NewClientWithOpts(docker_client.FromEnv, docker_client.WithAPIVersionNegotiation())
	require.NoError(t, err)

	_, err = c.Info(t.Context())
	if err != nil {
		t.Logf("Docker not available for test %s: %v", t.Name(), err)
		return false
	}
	return true
}
