package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/signing-service/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ImplementsInterface(t *testing.T) {
	client := New("http://localhost:8080")

	// Verify the client implements types.SigningService
	var _ types.SigningService = client
}

func TestClient_SignCreateDataSet(t *testing.T) {
	// Create a mock signature to return
	mockSignature := &eip712.AuthSignature{
		Signer: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		R:      common.BigToHash(big.NewInt(12345)),
		S:      common.BigToHash(big.NewInt(67890)),
		V:      27,
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sign/create-dataset", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode the request
		var req types.CreateDataSetRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "12345", req.ClientDataSetId)
		assert.Equal(t, "0xabCDEF1234567890ABcDEF1234567890aBCDeF12", req.Payee)
		assert.Len(t, req.Metadata, 1)
		assert.Equal(t, "test-key", req.Metadata[0].Key)

		// Return the mock signature
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockSignature)
	}))
	defer server.Close()

	// Create client
	client := New(server.URL)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	payee := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	metadata := []eip712.MetadataEntry{
		{Key: "test-key", Value: "test-value"},
	}

	signature, err := client.SignCreateDataSet(ctx, clientDataSetId, payee, metadata)
	require.NoError(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, mockSignature.Signer, signature.Signer)
	assert.Equal(t, mockSignature.R, signature.R)
	assert.Equal(t, mockSignature.S, signature.S)
	assert.Equal(t, mockSignature.V, signature.V)
}

func TestClient_SignAddPieces(t *testing.T) {
	// Create a mock signature to return
	mockSignature := &eip712.AuthSignature{
		Signer: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		R:      common.BigToHash(big.NewInt(12345)),
		S:      common.BigToHash(big.NewInt(67890)),
		V:      27,
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sign/add-pieces", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Decode the request
		var req types.AddPiecesRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "12345", req.ClientDataSetId)
		assert.Equal(t, "0", req.FirstAdded)
		assert.Len(t, req.PieceData, 2)

		// Verify hex-encoded piece data
		piece1, err := hex.DecodeString(req.PieceData[0])
		require.NoError(t, err)
		assert.Equal(t, []byte("piece1"), piece1)

		// Return the mock signature
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockSignature)
	}))
	defer server.Close()

	// Create client
	client := New(server.URL)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	firstAdded := big.NewInt(0)
	pieceData := [][]byte{
		[]byte("piece1"),
		[]byte("piece2"),
	}
	metadata := [][]eip712.MetadataEntry{
		{{Key: "piece1-key", Value: "piece1-value"}},
		{{Key: "piece2-key", Value: "piece2-value"}},
	}

	signature, err := client.SignAddPieces(ctx, clientDataSetId, firstAdded, pieceData, metadata)
	require.NoError(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, mockSignature.Signer, signature.Signer)
}

func TestClient_SignSchedulePieceRemovals(t *testing.T) {
	// Create a mock signature to return
	mockSignature := &eip712.AuthSignature{
		Signer: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		R:      common.BigToHash(big.NewInt(12345)),
		S:      common.BigToHash(big.NewInt(67890)),
		V:      27,
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sign/schedule-piece-removals", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Decode the request
		var req types.SchedulePieceRemovalsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "12345", req.ClientDataSetId)
		assert.Len(t, req.PieceIds, 3)
		assert.Equal(t, "1", req.PieceIds[0])
		assert.Equal(t, "2", req.PieceIds[1])
		assert.Equal(t, "3", req.PieceIds[2])

		// Return the mock signature
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockSignature)
	}))
	defer server.Close()

	// Create client
	client := New(server.URL)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	pieceIds := []*big.Int{
		big.NewInt(1),
		big.NewInt(2),
		big.NewInt(3),
	}

	signature, err := client.SignSchedulePieceRemovals(ctx, clientDataSetId, pieceIds)
	require.NoError(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, mockSignature.Signer, signature.Signer)
}

func TestClient_SignDeleteDataSet(t *testing.T) {
	// Create a mock signature to return
	mockSignature := &eip712.AuthSignature{
		Signer: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		R:      common.BigToHash(big.NewInt(12345)),
		S:      common.BigToHash(big.NewInt(67890)),
		V:      27,
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sign/delete-dataset", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Decode the request
		var req types.DeleteDataSetRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "12345", req.ClientDataSetId)

		// Return the mock signature
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockSignature)
	}))
	defer server.Close()

	// Create client
	client := New(server.URL)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)

	signature, err := client.SignDeleteDataSet(ctx, clientDataSetId)
	require.NoError(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, mockSignature.Signer, signature.Signer)
}

func TestClient_ServerError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	// Create client
	client := New(server.URL)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	payee := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	metadata := []eip712.MetadataEntry{}

	_, err := client.SignCreateDataSet(ctx, clientDataSetId, payee, metadata)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server returned status 500")
}

func TestClient_NetworkError(t *testing.T) {
	// Create client with invalid URL
	client := New("http://localhost:99999") // Port that should not be in use
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	payee := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	metadata := []eip712.MetadataEntry{}

	_, err := client.SignCreateDataSet(ctx, clientDataSetId, payee, metadata)
	require.Error(t, err)
}

func TestClient_CustomHTTPClient(t *testing.T) {
	// Create a custom HTTP client with a transport that always returns success
	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Header:     make(http.Header),
		},
	}
	customClient := &http.Client{Transport: mockTransport}

	client := NewWithHTTPClient("http://test.com", customClient)
	assert.NotNil(t, client)
	assert.Equal(t, customClient, client.httpClient)
}

// mockRoundTripper is a mock HTTP transport for testing
type mockRoundTripper struct {
	response *http.Response
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.response, nil
}
