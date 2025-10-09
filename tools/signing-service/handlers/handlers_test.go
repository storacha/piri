package handlers

import (
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/labstack/echo/v4"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/signing-service/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestSigner creates a test signer with a random key
func createTestSigner(t *testing.T) *eip712.Signer {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	chainID := big.NewInt(314159) // Calibration testnet
	contractAddr := common.HexToAddress("0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91")

	return eip712.NewSigner(privateKey, chainID, contractAddr)
}

func TestHealth(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Health(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, signer.GetAddress().Hex(), response["signer"])
}

func TestSignCreateDataSet_Success(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	// Use a valid checksummed address
	testPayee := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb")

	requestBody := types.CreateDataSetRequest{
		ClientDataSetId: "123",
		Payee:           testPayee.Hex(),
		Metadata: []eip712.MetadataEntry{
			{Key: "name", Value: "test-dataset"},
			{Key: "version", Value: "1.0"},
		},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/create-dataset", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignCreateDataSet(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)

	var signature eip712.AuthSignature
	err = json.Unmarshal(rec.Body.Bytes(), &signature)
	require.NoError(t, err)

	// Verify signature components
	assert.NotEmpty(t, signature.Signature)
	assert.Equal(t, signer.GetAddress(), signature.Signer)
	assert.NotEmpty(t, signature.SignedData)
	assert.True(t, signature.V == 27 || signature.V == 28, "V should be 27 or 28")
}

func TestSignCreateDataSet_InvalidJSON(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	req := httptest.NewRequest(http.MethodPost, "/sign/create-dataset", strings.NewReader("invalid json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.SignCreateDataSet(c)
	assert.Error(t, err)

	// Echo returns an HTTPError
	var he *echo.HTTPError
	ok := errors.As(err, &he)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSignCreateDataSet_InvalidDataSetId(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.CreateDataSetRequest{
		ClientDataSetId: "not-a-number",
		Payee:           "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
		Metadata:        []eip712.MetadataEntry{},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/create-dataset", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignCreateDataSet(c)
	assert.Error(t, err)

	var he *echo.HTTPError
	ok := errors.As(err, &he)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSignCreateDataSet_InvalidPayeeAddress(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.CreateDataSetRequest{
		ClientDataSetId: "123",
		Payee:           "not-an-address",
		Metadata:        []eip712.MetadataEntry{},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/create-dataset", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignCreateDataSet(c)
	assert.Error(t, err)

	var he *echo.HTTPError
	ok := errors.As(err, &he)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSignAddPieces_Success(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.AddPiecesRequest{
		ClientDataSetId: "123",
		FirstAdded:      "0",
		PieceData: []string{
			"0x0001020304",
			"0x0506070809",
		},
		Metadata: [][]eip712.MetadataEntry{
			{{Key: "size", Value: "1024"}},
			{{Key: "size", Value: "2048"}},
		},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/add-pieces", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignAddPieces(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)

	var signature eip712.AuthSignature
	err = json.Unmarshal(rec.Body.Bytes(), &signature)
	require.NoError(t, err)

	assert.NotEmpty(t, signature.Signature)
	assert.Equal(t, signer.GetAddress(), signature.Signer)
}

func TestSignAddPieces_InvalidDataSetId(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.AddPiecesRequest{
		ClientDataSetId: "invalid",
		FirstAdded:      "0",
		PieceData:       []string{},
		Metadata:        [][]eip712.MetadataEntry{},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/add-pieces", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignAddPieces(c)
	assert.Error(t, err)

	var he *echo.HTTPError
	ok := errors.As(err, &he)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSignAddPieces_InvalidFirstAdded(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.AddPiecesRequest{
		ClientDataSetId: "123",
		FirstAdded:      "not-a-number",
		PieceData:       []string{},
		Metadata:        [][]eip712.MetadataEntry{},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/add-pieces", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignAddPieces(c)
	assert.Error(t, err)

	var he *echo.HTTPError
	ok := errors.As(err, &he)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSignSchedulePieceRemovals_Success(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.SchedulePieceRemovalsRequest{
		ClientDataSetId: "123",
		PieceIds:        []string{"1", "2", "3"},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/schedule-piece-removals", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignSchedulePieceRemovals(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)

	var signature eip712.AuthSignature
	err = json.Unmarshal(rec.Body.Bytes(), &signature)
	require.NoError(t, err)

	assert.NotEmpty(t, signature.Signature)
	assert.Equal(t, signer.GetAddress(), signature.Signer)
}

func TestSignSchedulePieceRemovals_InvalidDataSetId(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.SchedulePieceRemovalsRequest{
		ClientDataSetId: "invalid",
		PieceIds:        []string{},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/schedule-piece-removals", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignSchedulePieceRemovals(c)
	assert.Error(t, err)

	var he *echo.HTTPError
	ok := errors.As(err, &he)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSignSchedulePieceRemovals_InvalidPieceId(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.SchedulePieceRemovalsRequest{
		ClientDataSetId: "123",
		PieceIds:        []string{"1", "invalid", "3"},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/schedule-piece-removals", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignSchedulePieceRemovals(c)
	assert.Error(t, err)

	var he *echo.HTTPError
	ok := errors.As(err, &he)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

func TestSignDeleteDataSet_Success(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.DeleteDataSetRequest{
		ClientDataSetId: "123",
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/delete-dataset", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignDeleteDataSet(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)

	var signature eip712.AuthSignature
	err = json.Unmarshal(rec.Body.Bytes(), &signature)
	require.NoError(t, err)

	assert.NotEmpty(t, signature.Signature)
	assert.Equal(t, signer.GetAddress(), signature.Signer)
}

func TestSignDeleteDataSet_InvalidDataSetId(t *testing.T) {
	e := echo.New()
	signer := createTestSigner(t)
	handler := NewHandler(signer)

	requestBody := types.DeleteDataSetRequest{
		ClientDataSetId: "not-a-number",
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/sign/delete-dataset", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.SignDeleteDataSet(c)
	assert.Error(t, err)

	var he *echo.HTTPError
	ok := errors.As(err, &he)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}

// TestSignatureConsistency verifies that signing the same data twice produces the same result
func TestSignatureConsistency(t *testing.T) {
	e := echo.New()

	// Create signer with fixed key for reproducibility
	privateKey, err := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)

	chainID := big.NewInt(314159)
	contractAddr := common.HexToAddress("0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91")
	signer := eip712.NewSigner(privateKey, chainID, contractAddr)
	handler := NewHandler(signer)

	requestBody := types.DeleteDataSetRequest{
		ClientDataSetId: "123",
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Sign first time
	req1 := httptest.NewRequest(http.MethodPost, "/sign/delete-dataset", strings.NewReader(string(body)))
	req1.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)

	err = handler.SignDeleteDataSet(c1)
	require.NoError(t, err)

	var sig1 eip712.AuthSignature
	err = json.Unmarshal(rec1.Body.Bytes(), &sig1)
	require.NoError(t, err)

	// Sign second time
	req2 := httptest.NewRequest(http.MethodPost, "/sign/delete-dataset", strings.NewReader(string(body)))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)

	err = handler.SignDeleteDataSet(c2)
	require.NoError(t, err)

	var sig2 eip712.AuthSignature
	err = json.Unmarshal(rec2.Body.Bytes(), &sig2)
	require.NoError(t, err)

	// Signatures should be identical for the same data
	assert.Equal(t, sig1.Signature, sig2.Signature)
	assert.Equal(t, sig1.V, sig2.V)
	assert.Equal(t, sig1.R, sig2.R)
	assert.Equal(t, sig1.S, sig2.S)
	assert.Equal(t, sig1.SignedData, sig2.SignedData)
}

// BenchmarkSignCreateDataSet benchmarks the CreateDataSet signing endpoint
func BenchmarkSignCreateDataSet(b *testing.B) {
	e := echo.New()
	signer := createTestSigner(&testing.T{})
	handler := NewHandler(signer)

	requestBody := types.CreateDataSetRequest{
		ClientDataSetId: "123",
		Payee:           "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
		Metadata: []eip712.MetadataEntry{
			{Key: "name", Value: "benchmark-dataset"},
		},
	}

	body, _ := json.Marshal(requestBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/sign/create-dataset", strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		require.NoError(b, handler.SignCreateDataSet(c))
	}
}
