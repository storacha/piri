package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/signing-service/types"
)

// Client implements types.SigningService using HTTP calls to a remote signing service
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Verify that Client implements types.SigningService at compile time
var _ types.SigningService = (*Client)(nil)

// New creates a new HTTP client for the signing service
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// NewWithHTTPClient creates a new HTTP client with a custom HTTP client
func NewWithHTTPClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// SignCreateDataSet signs a CreateDataSet operation via HTTP
func (c *Client) SignCreateDataSet(ctx context.Context,
	clientDataSetId *big.Int,
	payee common.Address,
	metadata []eip712.MetadataEntry) (*eip712.AuthSignature, error) {

	req := types.CreateDataSetRequest{
		ClientDataSetId: clientDataSetId.String(),
		Payee:           payee.Hex(),
		Metadata:        metadata,
	}

	var sig eip712.AuthSignature
	if err := c.post(ctx, "/sign/create-dataset", req, &sig); err != nil {
		return nil, fmt.Errorf("signing create dataset: %w", err)
	}

	return &sig, nil
}

// SignAddPieces signs an AddPieces operation via HTTP
func (c *Client) SignAddPieces(ctx context.Context,
	clientDataSetId *big.Int,
	firstAdded *big.Int,
	pieceData [][]byte,
	metadata [][]eip712.MetadataEntry) (*eip712.AuthSignature, error) {

	// Convert pieceData to hex strings
	pieceDataHex := make([]string, len(pieceData))
	for i, data := range pieceData {
		pieceDataHex[i] = hex.EncodeToString(data)
	}

	req := types.AddPiecesRequest{
		ClientDataSetId: clientDataSetId.String(),
		FirstAdded:      firstAdded.String(),
		PieceData:       pieceDataHex,
		Metadata:        metadata,
	}

	var sig eip712.AuthSignature
	if err := c.post(ctx, "/sign/add-pieces", req, &sig); err != nil {
		return nil, fmt.Errorf("signing add pieces: %w", err)
	}

	return &sig, nil
}

// SignSchedulePieceRemovals signs a SchedulePieceRemovals operation via HTTP
func (c *Client) SignSchedulePieceRemovals(ctx context.Context,
	clientDataSetId *big.Int,
	pieceIds []*big.Int) (*eip712.AuthSignature, error) {

	// Convert pieceIds to strings
	pieceIdsStr := make([]string, len(pieceIds))
	for i, id := range pieceIds {
		pieceIdsStr[i] = id.String()
	}

	req := types.SchedulePieceRemovalsRequest{
		ClientDataSetId: clientDataSetId.String(),
		PieceIds:        pieceIdsStr,
	}

	var sig eip712.AuthSignature
	if err := c.post(ctx, "/sign/schedule-piece-removals", req, &sig); err != nil {
		return nil, fmt.Errorf("signing schedule piece removals: %w", err)
	}

	return &sig, nil
}

// SignDeleteDataSet signs a DeleteDataSet operation via HTTP
func (c *Client) SignDeleteDataSet(ctx context.Context,
	clientDataSetId *big.Int) (*eip712.AuthSignature, error) {

	req := types.DeleteDataSetRequest{
		ClientDataSetId: clientDataSetId.String(),
	}

	var sig eip712.AuthSignature
	if err := c.post(ctx, "/sign/delete-dataset", req, &sig); err != nil {
		return nil, fmt.Errorf("signing delete dataset: %w", err)
	}

	return &sig, nil
}

// post makes an HTTP POST request to the signing service
func (c *Client) post(ctx context.Context, path string, reqBody interface{}, respBody interface{}) error {
	// Marshal request body
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	url := c.baseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Make HTTP request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Unmarshal response
	if err := json.Unmarshal(body, respBody); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}

	return nil
}
