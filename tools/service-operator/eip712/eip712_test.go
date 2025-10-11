package eip712

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEIP712Types(t *testing.T) {
	// Test that all required types are defined
	assert.Contains(t, EIP712Types, "EIP712Domain")
	assert.Contains(t, EIP712Types, "MetadataEntry")
	assert.Contains(t, EIP712Types, "CreateDataSet")
	assert.Contains(t, EIP712Types, "AddPieces")
	assert.Contains(t, EIP712Types, "Cid")
	assert.Contains(t, EIP712Types, "PieceMetadata")
	assert.Contains(t, EIP712Types, "SchedulePieceRemovals")
	assert.Contains(t, EIP712Types, "DeleteDataSet")
}

func TestGetDomain(t *testing.T) {
	chainId := big.NewInt(314159) // Filecoin Calibration
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	domain := GetDomain(chainId, contractAddr)

	assert.Equal(t, EIP712DomainName, domain.Name)
	assert.Equal(t, EIP712DomainVersion, domain.Version)
	// ChainId is stored as HexOrDecimal256, check it's not nil and matches
	assert.NotNil(t, domain.ChainId)
	// Convert back to big.Int for comparison
	assert.Equal(t, chainId.String(), (*big.Int)(domain.ChainId).String())
	assert.Equal(t, contractAddr.Hex(), domain.VerifyingContract)
}

func TestSignCreateDataSet(t *testing.T) {
	// Generate test private key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	chainId := big.NewInt(314159)
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	signer := NewSigner(privateKey, chainId, contractAddr)

	// Test data
	clientDataSetId := big.NewInt(0) // First dataset for this payer
	payee := common.HexToAddress("0xabcdef0123456789012345678901234567890123")
	metadata := []MetadataEntry{
		{Key: "test", Value: "value"},
		{Key: "with_cdn", Value: "true"},
	}

	// Sign
	authSig, err := signer.SignCreateDataSet(clientDataSetId, payee, metadata)
	require.NoError(t, err)

	// Verify signature components
	assert.NotNil(t, authSig.Signature)
	assert.Equal(t, 65, len(authSig.Signature))
	assert.True(t, authSig.V == 27 || authSig.V == 28)
	assert.NotEqual(t, common.Hash{}, authSig.R)
	assert.NotEqual(t, common.Hash{}, authSig.S)
	assert.Equal(t, signer.GetAddress(), authSig.Signer)
}

func TestEncodeExtraData(t *testing.T) {
	payer := common.HexToAddress("0x1234567890123456789012345678901234567890")
	keys := []string{"key1", "key2"}
	values := []string{"value1", "value2"}
	signature := []byte("test signature data")

	encoded, err := EncodeExtraDataForDataSetCreated(payer, keys, values, signature)
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	// Test mismatched keys and values
	_, err = EncodeExtraDataForDataSetCreated(payer, keys, []string{"value1"}, signature)
	assert.Error(t, err)
}

func TestCreateDataSetExtraData(t *testing.T) {
	// Generate test private key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	chainId := big.NewInt(314159)
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	signer := NewSigner(privateKey, chainId, contractAddr)

	// Test data
	clientDataSetId := big.NewInt(0) // First dataset for this payer
	payee := common.HexToAddress("0xabcdef0123456789012345678901234567890123")
	metadata := []MetadataEntry{
		{Key: "service", Value: "storacha"},
		{Key: "with_cdn", Value: "false"},
	}

	extraData, err := CreateDataSetExtraData(signer, clientDataSetId, payee, metadata)
	require.NoError(t, err)
	assert.NotEmpty(t, extraData)

	// The extraData should be ABI encoded
	t.Logf("ExtraData (hex): 0x%s", hex.EncodeToString(extraData))
}

func TestSignAddPieces(t *testing.T) {
	// Generate test private key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	chainId := big.NewInt(314159)
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	signer := NewSigner(privateKey, chainId, contractAddr)

	// Test data
	clientDataSetId := big.NewInt(123)
	firstAdded := big.NewInt(1)
	pieceData := [][]byte{
		[]byte("piece1"),
		[]byte("piece2"),
	}
	metadata := [][]MetadataEntry{
		{{Key: "name", Value: "piece1"}},
		{{Key: "name", Value: "piece2"}},
	}

	// Sign
	authSig, err := signer.SignAddPieces(clientDataSetId, firstAdded, pieceData, metadata)
	require.NoError(t, err)

	// Verify signature components
	assert.NotNil(t, authSig.Signature)
	assert.Equal(t, 65, len(authSig.Signature))
	assert.True(t, authSig.V == 27 || authSig.V == 28)
	assert.NotEqual(t, common.Hash{}, authSig.R)
	assert.NotEqual(t, common.Hash{}, authSig.S)
	assert.Equal(t, signer.GetAddress(), authSig.Signer)
}
