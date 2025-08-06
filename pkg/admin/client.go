
package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client is a client for the admin API.
type Client struct {
	addr   string
	http   *http.Client
	scheme string
}

// NewClient creates a new admin client.
func NewClient(addr string) *Client {
	return &Client{
		addr:   addr,
		http:   http.DefaultClient,
		scheme: "http",
	}
}

// ListLogLevels lists the log levels of all subsystems.
func (c *Client) ListLogLevels(ctx context.Context) (*ListLogLevelsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("/log/level"), nil)
	if err != nil {
		return nil, err
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var out ListLogLevelsResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// SetLogLevel sets the log level of a subsystem.
func (c *Client) SetLogLevel(ctx context.Context, subsystem, level string) error {
	breq := SetLogLevelRequest{
		Subsystem: subsystem,
		Level:     level,
	}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(breq); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/log/level"), buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (c *Client) url(path string) string {
	return fmt.Sprintf("%s://%s%s", c.scheme, c.addr, path)
}
