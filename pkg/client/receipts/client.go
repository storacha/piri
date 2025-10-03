package receipts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/core/message"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/transport"
	"github.com/storacha/go-ucanto/transport/car"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
)

var ErrNotFound = errors.New("receipt not found")

type Client struct {
	endpoint *url.URL
	client   *http.Client
	codec    transport.ResponseDecoder
}

type Option func(c *Client)

func WithCodec(codec transport.ResponseDecoder) Option {
	return func(c *Client) {
		c.codec = codec
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.client = client
	}
}

func NewClient(endpoint *url.URL, options ...Option) *Client {
	c := Client{
		endpoint: endpoint,
		codec:    car.NewOutboundCodec(),
	}
	for _, o := range options {
		o(&c)
	}
	if c.client == nil {
		c.client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	return &c
}

// Fetch a receipt from the receipt API. Returns [ErrNotFound] if the API
// responds with [http.StatusNotFound].
func (c *Client) Fetch(ctx context.Context, lnk ucan.Link) (receipt.AnyReceipt, error) {
	receiptURL := c.endpoint.JoinPath(lnk.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, receiptURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating get request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doing receipts request: %w", err)
	}
	defer resp.Body.Close()

	var msg message.AgentMessage
	switch resp.StatusCode {
	case http.StatusOK:
		msg, err = c.codec.Decode(ucanhttp.NewResponse(resp.StatusCode, resp.Body, resp.Header))
		if err != nil {
			return nil, fmt.Errorf("decoding message: %w", err)
		}
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	rcptlnk, ok := msg.Get(lnk)
	if !ok {
		return nil, errors.New("receipt not found in agent message")
	}

	reader := receipt.NewAnyReceiptReader(types.Converters...)
	return reader.Read(rcptlnk, msg.Blocks())
}
