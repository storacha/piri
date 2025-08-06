
package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/storacha/piri/pkg/admin"
)

func TestLogListCmd(t *testing.T) {
	expected := map[string]string{
		"system1": "INFO",
		"system2": "DEBUG",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/log/level", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		resp := admin.ListLogLevelsResponse{
			Levels: expected,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"log", "list", "--manage-api", strings.TrimPrefix(server.URL, "http://")})
	require.NoError(t, rootCmd.Execute())

	// restore stdout
	w.Close()
	os.Stdout = old

	var out bytes.Buffer
	io.Copy(&out, r)

	for k, v := range expected {
		require.Contains(t, out.String(), k)
		require.Contains(t, out.String(), v)
	}
}

func TestLogSetLevelCmd(t *testing.T) {
	t.Run("sets level for a single system", func(t *testing.T) {
		rootCmd := newRootCmd()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/log/level", r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)

			var req admin.SetLogLevelRequest
			json.NewDecoder(r.Body).Decode(&req)

			require.Equal(t, "system1", req.Subsystem)
			require.Equal(t, "DEBUG", req.Level)
		}))
		defer server.Close()

		rootCmd.SetArgs([]string{"log", "set-level", "--system", "system1", "DEBUG", "--manage-api", strings.TrimPrefix(server.URL, "http://")})
		require.NoError(t, rootCmd.Execute())
	})

	t.Run("sets level for multiple systems", func(t *testing.T) {
		rootCmd := newRootCmd()
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			require.Equal(t, "/log/level", r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)

			var req admin.SetLogLevelRequest
			json.NewDecoder(r.Body).Decode(&req)

			require.Contains(t, []string{"system1", "system2"}, req.Subsystem)
			require.Equal(t, "WARN", req.Level)
		}))
		defer server.Close()

		rootCmd.SetArgs([]string{"log", "set-level", "--system", "system1", "--system", "system2", "WARN", "--manage-api", strings.TrimPrefix(server.URL, "http://")})
		require.NoError(t, rootCmd.Execute())
		require.Equal(t, 2, requests)
	})

	t.Run("sets level for all systems", func(t *testing.T) {
		rootCmd := newRootCmd()
		postRequests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/log/level", r.URL.Path)

			if r.Method == http.MethodGet {
				resp := admin.ListLogLevelsResponse{
					Levels: map[string]string{"system1": "INFO", "system2": "INFO"},
				}
				json.NewEncoder(w).Encode(resp)
				return
			}

			postRequests++
			require.Equal(t, http.MethodPost, r.Method)

			var req admin.SetLogLevelRequest
			json.NewDecoder(r.Body).Decode(&req)

			require.Contains(t, []string{"system1", "system2"}, req.Subsystem)
			require.Equal(t, "FATAL", req.Level)
		}))
		defer server.Close()

		rootCmd.SetArgs([]string{"log", "set-level", "FATAL", "--manage-api", strings.TrimPrefix(server.URL, "http://")})
		require.NoError(t, rootCmd.Execute())
		require.Equal(t, 2, postRequests)
	})
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "piri",
		Short: "Piri is the software run by all storage providers on the Storacha network",
	}
	root.AddCommand(NewLogCmd())
	return root
}
