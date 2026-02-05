package client

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/lib"
	"github.com/storacha/piri/pkg/admin/httpapi"
	"github.com/storacha/piri/pkg/config"
)

type Client struct {
	endpoint   *url.URL
	httpClient *http.Client
	authHeader string
}

type Option func(*Client) error

// WithHTTPClient replaces the underlying HTTP client (for custom timeouts, tracing, etc.).
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) error {
		if client == nil {
			return fmt.Errorf("http client cannot be nil")
		}
		c.httpClient = client
		return nil
	}
}

// WithBearerFromSigner configures the Authorization header using a JWT signed by the provided signer.
func WithBearerFromSigner(id principal.Signer) Option {
	return func(c *Client) error {
		authHeader, err := createAuthBearerTokenFromID(id)
		if err != nil {
			return fmt.Errorf("creating auth header from signer: %w", err)
		}
		c.authHeader = authHeader
		return nil
	}
}

// New constructs an admin API client.
func New(endpoint *url.URL, opts ...Option) (*Client, error) {
	// Keep endpoint required to avoid nil dereference at call sites.
	if endpoint == nil {
		return nil, fmt.Errorf("endpoint is required")
	}

	c := &Client{
		endpoint: endpoint,
		// Default client with a sane timeout to avoid hanging calls.
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// NewFromConfig builds a client using repository config defaults.
func NewFromConfig(cfg config.Client) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating client config: %w", err)
	}

	endpoint, err := url.Parse(cfg.API.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing admin api endpoint: %w", err)
	}

	id, err := lib.SignerFromEd25519PEMFile(cfg.Identity.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading identity key file: %w", err)
	}

	return New(endpoint, WithBearerFromSigner(id))
}

// ListLogLevels fetches the list of configured loggers and their levels.
func (c *Client) ListLogLevels(ctx context.Context) (map[string]string, error) {
	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.LogRoutePath + "/list").String()

	var resp httpapi.ListLogLevelsResponse
	if err := c.getJSON(ctx, route, &resp); err != nil {
		return nil, err
	}

	return resp.Loggers, nil
}

// SetLogLevel sets the log level for a specific subsystem.
func (c *Client) SetLogLevel(ctx context.Context, system, level string) error {
	if system == "" {
		return fmt.Errorf("system is required")
	}
	if level == "" {
		return fmt.Errorf("level is required")
	}

	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.LogRoutePath + "/set").String()
	req := httpapi.SetLogLevelRequest{
		System: system,
		Level:  level,
	}

	return c.verifySuccess(c.postJSON(ctx, route, req))
}

// SetLogLevelRegex sets the log level for all subsystems matching the expression.
func (c *Client) SetLogLevelRegex(ctx context.Context, expression, level string) error {
	if expression == "" {
		return fmt.Errorf("expression is required")
	}
	if level == "" {
		return fmt.Errorf("level is required")
	}

	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.LogRoutePath + "/set-regex").String()
	req := httpapi.SetLogLevelRegexRequest{
		Expression: expression,
		Level:      level,
	}

	return c.verifySuccess(c.postJSON(ctx, route, req))
}

// GetAccountInfo fetches the payment account information for the storage operator.
func (c *Client) GetAccountInfo(ctx context.Context) (*httpapi.GetAccountInfoResponse, error) {
	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.PaymentRoutePath + "/account").String()

	var resp httpapi.GetAccountInfoResponse
	if err := c.getJSON(ctx, route, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// EstimateSettlement returns estimated gas and fees for settling a rail.
func (c *Client) EstimateSettlement(ctx context.Context, railID string) (*httpapi.EstimateSettlementResponse, error) {
	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.PaymentRoutePath + "/settle/" + railID + "/estimate").String()

	var resp httpapi.EstimateSettlementResponse
	if err := c.getJSON(ctx, route, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// SettleRail submits a settlement transaction for a rail.
func (c *Client) SettleRail(ctx context.Context, railID string) (*httpapi.SettleRailResponse, error) {
	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.PaymentRoutePath + "/settle/" + railID).String()

	res, err := c.postJSON(ctx, route, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, errFromResponse(res)
	}

	var resp httpapi.SettleRailResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decoding response JSON: %w", err)
	}

	return &resp, nil
}

// GetSettlementStatus returns the status of a pending settlement for a rail.
func (c *Client) GetSettlementStatus(ctx context.Context, railID string) (*httpapi.SettlementStatusResponse, error) {
	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.PaymentRoutePath + "/settle/" + railID + "/status").String()

	var resp httpapi.SettlementStatusResponse
	if err := c.getJSON(ctx, route, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// EstimateWithdraw returns estimated gas and fees for a withdrawal.
func (c *Client) EstimateWithdraw(ctx context.Context, recipient, amount string) (*httpapi.EstimateWithdrawResponse, error) {
	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.PaymentRoutePath + "/withdraw/estimate").String()

	req := httpapi.EstimateWithdrawRequest{
		Recipient: recipient,
		Amount:    amount,
	}

	res, err := c.postJSON(ctx, route, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, errFromResponse(res)
	}

	var resp httpapi.EstimateWithdrawResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decoding response JSON: %w", err)
	}

	return &resp, nil
}

// Withdraw submits a withdrawal transaction.
func (c *Client) Withdraw(ctx context.Context, recipient, amount string) (*httpapi.WithdrawResponse, error) {
	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.PaymentRoutePath + "/withdraw").String()

	req := httpapi.WithdrawRequest{
		Recipient: recipient,
		Amount:    amount,
	}

	res, err := c.postJSON(ctx, route, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, errFromResponse(res)
	}

	var resp httpapi.WithdrawResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decoding response JSON: %w", err)
	}

	return &resp, nil
}

// GetWithdrawalStatus returns the status of a pending withdrawal.
func (c *Client) GetWithdrawalStatus(ctx context.Context) (*httpapi.WithdrawalStatusResponse, error) {
	route := c.endpoint.JoinPath(httpapi.AdminRoutePath + httpapi.PaymentRoutePath + "/withdraw/status").String()

	var resp httpapi.WithdrawalStatusResponse
	if err := c.getJSON(ctx, route, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func createAuthBearerTokenFromID(id principal.Signer) (string, error) {
	claims := jwt.MapClaims{
		"service_name": "storacha",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)

	tokenString, err := token.SignedString(ed25519.PrivateKey(id.Raw()))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	return "Bearer " + tokenString, nil
}

func (c *Client) sendRequest(ctx context.Context, method string, url string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}

	if c.authHeader != "" {
		req.Header.Add("Authorization", c.authHeader)
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	// Only set content-type when we have a body to avoid surprising intermediaries.
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	return res, nil
}

func (c *Client) postJSON(ctx context.Context, url string, params interface{}) (*http.Response, error) {
	var body io.Reader
	if params != nil {
		asBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("encoding request parameters: %w", err)
		}
		body = bytes.NewReader(asBytes)
	}

	return c.sendRequest(ctx, http.MethodPost, url, body, nil)
}

func (c *Client) getJSON(ctx context.Context, url string, target interface{}) error {
	res, err := c.sendRequest(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return errFromResponse(res)
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return fmt.Errorf("decoding response JSON: %w", err)
	}
	return nil
}

func (c *Client) verifySuccess(res *http.Response, err error) error {
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return errFromResponse(res)
	}
	return nil
}

type ErrFailedResponse struct {
	StatusCode int
	Body       string
}

func errFromResponse(res *http.Response) ErrFailedResponse {
	err := ErrFailedResponse{StatusCode: res.StatusCode}

	message, merr := io.ReadAll(res.Body)
	if merr != nil {
		err.Body = merr.Error()
	} else {
		err.Body = string(message)
	}
	return err
}

func (e ErrFailedResponse) Error() string {
	return fmt.Sprintf("http request received unexpected status: %d %s, message: %s", e.StatusCode, http.StatusText(e.StatusCode), e.Body)
}
