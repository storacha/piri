package shutdown

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var log = logging.Logger("cmd/shutdown")

var Cmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Gracefully shutdown a running piri server",
	Long: `Send a shutdown signal to a running piri server instance.
The server will perform a graceful shutdown, completing any in-progress operations.`,
	Args: cobra.NoArgs,
	RunE: shutdownServer,
}

func init() {
	Cmd.Flags().String(
		"host",
		"localhost",
		"Host where the piri server is running",
	)
	cobra.CheckErr(viper.BindPFlag("shutdown.host", Cmd.Flags().Lookup("host")))

	Cmd.Flags().Uint(
		"port",
		3000,
		"Port where the piri server is listening",
	)
	cobra.CheckErr(viper.BindPFlag("shutdown.port", Cmd.Flags().Lookup("port")))

	Cmd.Flags().Duration(
		"timeout",
		10*time.Second,
		"Timeout for the shutdown request",
	)
	cobra.CheckErr(viper.BindPFlag("shutdown.timeout", Cmd.Flags().Lookup("timeout")))
}

func shutdownServer(cmd *cobra.Command, _ []string) error {
	host := viper.GetString("shutdown.host")
	port := viper.GetUint("shutdown.port")
	timeout := viper.GetDuration("shutdown.timeout")

	// Construct the shutdown endpoint URL
	url := fmt.Sprintf("http://%s:%d/admin/shutdown", host, port)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Create shutdown request
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, url, bytes.NewReader([]byte{}))
	if err != nil {
		return fmt.Errorf("creating shutdown request: %w", err)
	}

	cmd.Printf("Sending shutdown signal to piri server at %s:%d...\n", host, port)

	// Send shutdown request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send shutdown request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		cmd.Println("Shutdown signal sent successfully. Server is shutting down gracefully.")
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("shutdown endpoint not found. Is the server running with admin routes enabled?")
	case http.StatusServiceUnavailable:
		return fmt.Errorf("server is already shutting down")
	default:
		return fmt.Errorf("unexpected response from server: %s", resp.Status)
	}
}