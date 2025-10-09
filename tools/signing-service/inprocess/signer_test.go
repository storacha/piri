package inprocess

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/signing-service/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestSigner(t *testing.T) (*Signer, *ecdsa.PrivateKey) {
	// Generate a test private key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create a test contract address
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Create a test chain ID
	chainID := big.NewInt(31415926)

	// Create the eip712 signer
	eip712Signer := eip712.NewSigner(privateKey, chainID, contractAddr)

	// Create the in-process signer
	signer := New(eip712Signer)

	return signer, privateKey
}

func TestSigner_ImplementsInterface(t *testing.T) {
	signer, _ := setupTestSigner(t)

	// Verify the signer implements types.SigningService
	var _ types.SigningService = signer
}

func TestSigner_SignCreateDataSet(t *testing.T) {
	signer, privateKey := setupTestSigner(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	payee := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	metadata := []eip712.MetadataEntry{
		{Key: "test-key", Value: "test-value"},
	}

	signature, err := signer.SignCreateDataSet(ctx, clientDataSetId, payee, metadata)
	require.NoError(t, err)
	assert.NotNil(t, signature)

	// Verify the signature has the correct signer address
	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	assert.Equal(t, expectedAddress, signature.Signer)

	// Verify signature fields are populated
	assert.NotEmpty(t, signature.R)
	assert.NotEmpty(t, signature.S)
	assert.NotZero(t, signature.V)
}

func TestSigner_SignAddPieces(t *testing.T) {
	signer, privateKey := setupTestSigner(t)
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

	signature, err := signer.SignAddPieces(ctx, clientDataSetId, firstAdded, pieceData, metadata)
	require.NoError(t, err)
	assert.NotNil(t, signature)

	// Verify the signature has the correct signer address
	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	assert.Equal(t, expectedAddress, signature.Signer)

	// Verify signature fields are populated
	assert.NotEmpty(t, signature.R)
	assert.NotEmpty(t, signature.S)
	assert.NotZero(t, signature.V)
}

func TestSigner_SignSchedulePieceRemovals(t *testing.T) {
	signer, privateKey := setupTestSigner(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	pieceIds := []*big.Int{
		big.NewInt(1),
		big.NewInt(2),
		big.NewInt(3),
	}

	signature, err := signer.SignSchedulePieceRemovals(ctx, clientDataSetId, pieceIds)
	require.NoError(t, err)
	assert.NotNil(t, signature)

	// Verify the signature has the correct signer address
	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	assert.Equal(t, expectedAddress, signature.Signer)

	// Verify signature fields are populated
	assert.NotEmpty(t, signature.R)
	assert.NotEmpty(t, signature.S)
	assert.NotZero(t, signature.V)
}

func TestSigner_SignDeleteDataSet(t *testing.T) {
	signer, privateKey := setupTestSigner(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)

	signature, err := signer.SignDeleteDataSet(ctx, clientDataSetId)
	require.NoError(t, err)
	assert.NotNil(t, signature)

	// Verify the signature has the correct signer address
	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	assert.Equal(t, expectedAddress, signature.Signer)

	// Verify signature fields are populated
	assert.NotEmpty(t, signature.R)
	assert.NotEmpty(t, signature.S)
	assert.NotZero(t, signature.V)
}

func TestSigner_SignatureConsistency(t *testing.T) {
	signer, _ := setupTestSigner(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	payee := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	metadata := []eip712.MetadataEntry{
		{Key: "test-key", Value: "test-value"},
	}

	// Sign the same data twice
	sig1, err := signer.SignCreateDataSet(ctx, clientDataSetId, payee, metadata)
	require.NoError(t, err)

	sig2, err := signer.SignCreateDataSet(ctx, clientDataSetId, payee, metadata)
	require.NoError(t, err)

	// Signatures should be identical for the same input
	assert.Equal(t, sig1.R, sig2.R)
	assert.Equal(t, sig1.S, sig2.S)
	assert.Equal(t, sig1.V, sig2.V)
	assert.Equal(t, sig1.Signer, sig2.Signer)
}
