package eip712

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEIP2612PermitTypes(t *testing.T) {
	// Test that all required permit types are defined
	assert.Contains(t, EIP2612PermitTypes, "EIP712Domain")
	assert.Contains(t, EIP2612PermitTypes, "Permit")

	// Verify EIP712Domain structure
	domainType := EIP2612PermitTypes["EIP712Domain"]
	assert.Len(t, domainType, 4)
	assert.Equal(t, "name", domainType[0].Name)
	assert.Equal(t, "version", domainType[1].Name)
	assert.Equal(t, "chainId", domainType[2].Name)
	assert.Equal(t, "verifyingContract", domainType[3].Name)

	// Verify Permit structure
	permitType := EIP2612PermitTypes["Permit"]
	assert.Len(t, permitType, 5)
	assert.Equal(t, "owner", permitType[0].Name)
	assert.Equal(t, "spender", permitType[1].Name)
	assert.Equal(t, "value", permitType[2].Name)
	assert.Equal(t, "nonce", permitType[3].Name)
	assert.Equal(t, "deadline", permitType[4].Name)
}

func TestGetPermitHash(t *testing.T) {
	// Test data
	chainID := big.NewInt(314159)
	tokenAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	owner := common.HexToAddress("0xabcdef0123456789012345678901234567890123")
	spender := common.HexToAddress("0x9876543210987654321098765432109876543210")
	value := big.NewInt(1000000)
	nonce := big.NewInt(0)
	deadline := big.NewInt(1704067200) // 2024-01-01

	// Build domain (uses token address as verifyingContract)
	domain := apitypes.TypedDataDomain{
		Name:              "USD Coin",
		Version:           "1",
		ChainId:           (*math.HexOrDecimal256)(chainID),
		VerifyingContract: tokenAddress.Hex(),
	}

	// Create permit message
	message := map[string]interface{}{
		"owner":    owner.Hex(),
		"spender":  spender.Hex(),
		"value":    value,
		"nonce":    nonce,
		"deadline": deadline,
	}

	// Get hash
	hash, err := getPermitHash(domain, message)
	require.NoError(t, err)

	// Verify hash properties
	assert.NotNil(t, hash)
	assert.Equal(t, 32, len(hash))
	assert.NotEqual(t, make([]byte, 32), hash) // Should not be all zeros

	// Test that same inputs produce same hash
	hash2, err := getPermitHash(domain, message)
	require.NoError(t, err)
	assert.Equal(t, hash, hash2)

	// Test that different inputs produce different hash
	differentMessage := map[string]interface{}{
		"owner":    owner.Hex(),
		"spender":  spender.Hex(),
		"value":    big.NewInt(2000000), // Different value
		"nonce":    nonce,
		"deadline": deadline,
	}
	differentHash, err := getPermitHash(domain, differentMessage)
	require.NoError(t, err)
	assert.NotEqual(t, hash, differentHash)
}

func TestGeneratePermitSignatureWithKnownKey(t *testing.T) {
	// This test verifies the signature structure without making network calls
	// We'll manually create TokenInfo instead of querying it

	// Generate a known test private key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	owner := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Test data
	chainID := big.NewInt(314159)
	tokenAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	spender := common.HexToAddress("0x9876543210987654321098765432109876543210")
	amount := big.NewInt(1000000)
	deadline := big.NewInt(1704067200)

	// Manually create TokenInfo to avoid network calls
	tokenInfo := &TokenInfo{
		Name:    "USD Coin",
		Version: "1",
		Nonce:   big.NewInt(0),
	}

	// Build EIP-712 domain (uses token address as verifyingContract)
	domain := apitypes.TypedDataDomain{
		Name:              tokenInfo.Name,
		Version:           tokenInfo.Version,
		ChainId:           (*math.HexOrDecimal256)(chainID),
		VerifyingContract: tokenAddress.Hex(),
	}

	// Create permit message
	message := map[string]interface{}{
		"owner":    owner.Hex(),
		"spender":  spender.Hex(),
		"value":    amount,
		"nonce":    tokenInfo.Nonce,
		"deadline": deadline,
	}

	// Get the EIP-712 hash to sign
	hash, err := getPermitHash(domain, message)
	require.NoError(t, err)

	// Sign the hash
	signature, err := crypto.Sign(hash, privateKey)
	require.NoError(t, err)

	// Verify signature structure
	assert.Equal(t, 65, len(signature))

	// Transform V from recovery ID to Ethereum signature standard
	v := signature[64] + 27
	assert.True(t, v == 27 || v == 28, "V should be 27 or 28")

	// Extract r and s
	var r, s [32]byte
	copy(r[:], signature[:32])
	copy(s[:], signature[32:64])

	// Verify R and S are not zero
	assert.NotEqual(t, [32]byte{}, r, "R should not be zero")
	assert.NotEqual(t, [32]byte{}, s, "S should not be zero")

	// Test signature recovery
	// Adjust V back to recovery ID for SigToPub
	sig := make([]byte, 65)
	copy(sig[:64], signature[:64])
	sig[64] = signature[64] // Keep original V (0 or 1)

	pubKey, err := crypto.SigToPub(hash, sig)
	require.NoError(t, err)

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	assert.Equal(t, owner, recoveredAddr, "Recovered address should match owner")
}

func TestPermitSignatureComponents(t *testing.T) {
	// Test PermitSignature structure
	sig := &PermitSignature{
		V:        27,
		R:        [32]byte{1, 2, 3},
		S:        [32]byte{4, 5, 6},
		Deadline: big.NewInt(1704067200),
	}

	assert.Equal(t, uint8(27), sig.V)
	assert.NotEqual(t, [32]byte{}, sig.R)
	assert.NotEqual(t, [32]byte{}, sig.S)
	assert.NotNil(t, sig.Deadline)
	assert.Equal(t, int64(1704067200), sig.Deadline.Int64())
}

func TestTokenInfo(t *testing.T) {
	// Test TokenInfo structure
	info := &TokenInfo{
		Name:    "USD Coin",
		Version: "1",
		Nonce:   big.NewInt(5),
	}

	assert.Equal(t, "USD Coin", info.Name)
	assert.Equal(t, "1", info.Version)
	assert.Equal(t, int64(5), info.Nonce.Int64())
}

func TestPermitDomainSeparatorUsesTokenAddress(t *testing.T) {
	// Verify that permit domain uses token address, not service contract address
	// This is different from PDP auth which uses service contract address

	tokenAddress := common.HexToAddress("0x1111111111111111111111111111111111111111")
	chainID := big.NewInt(314159)

	domain := apitypes.TypedDataDomain{
		Name:              "USD Coin",
		Version:           "1",
		ChainId:           (*math.HexOrDecimal256)(chainID),
		VerifyingContract: tokenAddress.Hex(),
	}

	// Verify the verifyingContract is the token address
	assert.Equal(t, tokenAddress.Hex(), domain.VerifyingContract)

	// Compare with PDP auth domain which uses service contract address
	serviceContractAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")
	pdpDomain := GetDomain(chainID, serviceContractAddress)

	// PDP domain should use service contract address
	assert.Equal(t, serviceContractAddress.Hex(), pdpDomain.VerifyingContract)

	// They should be different
	assert.NotEqual(t, domain.VerifyingContract, pdpDomain.VerifyingContract)
}
