package config

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkDefaults_Calibration(t *testing.T) {
	rpcURL, contractAddr, err := NetworkDefaults(NetworkCalibration)

	require.NoError(t, err)
	assert.Equal(t, DefaultRpcUrl, rpcURL)
	assert.Equal(t, DefaultContractAddress, contractAddr)
	assert.True(t, common.IsHexAddress(contractAddr))
}

func TestNetworkDefaults_Mainnet(t *testing.T) {
	_, _, err := NetworkDefaults(NetworkMainnet)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet supported")
}

func TestNetworkDefaults_InvalidNetwork(t *testing.T) {
	_, _, err := NetworkDefaults("invalid-network")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown network")
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		RPCUrl:          "https://api.calibration.node.glif.io/rpc/v1",
		ContractAddress: "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91",
		PrivateKeyPath:  "/path/to/key.hex",
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingRPCUrl(t *testing.T) {
	cfg := &Config{
		ContractAddress: "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91",
		PrivateKeyPath:  "/path/to/key.hex",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rpc-url is required")
}

func TestConfig_Validate_MissingContractAddress(t *testing.T) {
	cfg := &Config{
		RPCUrl:         "https://api.calibration.node.glif.io/rpc/v1",
		PrivateKeyPath: "/path/to/key.hex",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contract-address is required")
}

func TestConfig_Validate_InvalidContractAddress(t *testing.T) {
	cfg := &Config{
		RPCUrl:          "https://api.calibration.node.glif.io/rpc/v1",
		ContractAddress: "not-a-valid-address",
		PrivateKeyPath:  "/path/to/key.hex",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contract address")
}

func TestConfig_Validate_MissingPrivateKeyAndKeystore(t *testing.T) {
	cfg := &Config{
		RPCUrl:          "https://api.calibration.node.glif.io/rpc/v1",
		ContractAddress: "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either private-key or keystore must be provided")
}

func TestConfig_Validate_KeystoreWithoutPassword(t *testing.T) {
	cfg := &Config{
		RPCUrl:          "https://api.calibration.node.glif.io/rpc/v1",
		ContractAddress: "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91",
		KeystorePath:    "/path/to/keystore.json",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "keystore-password is required")
}

func TestConfig_Validate_KeystoreWithPassword(t *testing.T) {
	cfg := &Config{
		RPCUrl:           "https://api.calibration.node.glif.io/rpc/v1",
		ContractAddress:  "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91",
		KeystorePath:     "/path/to/keystore.json",
		KeystorePassword: "password123",
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_ContractAddr(t *testing.T) {
	cfg := &Config{
		ContractAddress: "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91",
	}

	addr := cfg.ContractAddr()
	assert.Equal(t, "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91", addr.Hex())
}

func TestLoadPrivateKey_HexFormat(t *testing.T) {
	// Create temporary file with hex-encoded private key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	keyBytes := crypto.FromECDSA(privateKey)
	keyHex := hex.EncodeToString(keyBytes)

	tempFile := filepath.Join(t.TempDir(), "key.hex")
	err = os.WriteFile(tempFile, []byte(keyHex), 0600)
	require.NoError(t, err)

	// Load the key
	loadedKey, err := LoadPrivateKey(tempFile)
	require.NoError(t, err)
	assert.Equal(t, privateKey.D, loadedKey.D)
}

func TestLoadPrivateKey_HexFormatWith0xPrefix(t *testing.T) {
	// Create temporary file with 0x-prefixed hex key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	keyBytes := crypto.FromECDSA(privateKey)
	keyHex := "0x" + hex.EncodeToString(keyBytes)

	tempFile := filepath.Join(t.TempDir(), "key.hex")
	err = os.WriteFile(tempFile, []byte(keyHex), 0600)
	require.NoError(t, err)

	// Load the key
	loadedKey, err := LoadPrivateKey(tempFile)
	require.NoError(t, err)
	assert.Equal(t, privateKey.D, loadedKey.D)
}

func TestLoadPrivateKey_HexFormatWithWhitespace(t *testing.T) {
	// Create temporary file with whitespace around key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	keyBytes := crypto.FromECDSA(privateKey)
	keyHex := "  \n" + hex.EncodeToString(keyBytes) + "  \n"

	tempFile := filepath.Join(t.TempDir(), "key.hex")
	err = os.WriteFile(tempFile, []byte(keyHex), 0600)
	require.NoError(t, err)

	// Load the key
	loadedKey, err := LoadPrivateKey(tempFile)
	require.NoError(t, err)
	assert.Equal(t, privateKey.D, loadedKey.D)
}

func TestLoadPrivateKey_FileNotFound(t *testing.T) {
	_, err := LoadPrivateKey("/nonexistent/path/key.hex")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading private key file")
}

func TestLoadPrivateKey_InvalidKey(t *testing.T) {
	// Create temporary file with invalid key
	tempFile := filepath.Join(t.TempDir(), "invalid.hex")
	err := os.WriteFile(tempFile, []byte("invalid-key-data"), 0600)
	require.NoError(t, err)

	_, err = LoadPrivateKey(tempFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing private key")
}

func TestLoadPrivateKeyFromKeystore_Success(t *testing.T) {
	// Generate a test key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create keystore file
	ks := keystore.NewKeyStore(t.TempDir(), keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.ImportECDSA(privateKey, "test-password")
	require.NoError(t, err)

	// Get the keystore file path
	keystorePath := account.URL.Path

	// Load the key from keystore
	loadedKey, err := LoadPrivateKeyFromKeystore(keystorePath, "test-password")
	require.NoError(t, err)
	assert.Equal(t, privateKey.D, loadedKey.D)
}

func TestLoadPrivateKeyFromKeystore_WrongPassword(t *testing.T) {
	// Generate a test key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create keystore file
	ks := keystore.NewKeyStore(t.TempDir(), keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.ImportECDSA(privateKey, "correct-password")
	require.NoError(t, err)

	keystorePath := account.URL.Path

	// Try to load with wrong password
	_, err = LoadPrivateKeyFromKeystore(keystorePath, "wrong-password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decrypting keystore")
}

func TestLoadPrivateKeyFromKeystore_FileNotFound(t *testing.T) {
	_, err := LoadPrivateKeyFromKeystore("/nonexistent/keystore.json", "password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading keystore file")
}

func TestLoadPrivateKeyFromKeystore_InvalidJSON(t *testing.T) {
	// Create invalid keystore file
	tempFile := filepath.Join(t.TempDir(), "invalid-keystore.json")
	err := os.WriteFile(tempFile, []byte("not valid json"), 0600)
	require.NoError(t, err)

	_, err = LoadPrivateKeyFromKeystore(tempFile, "password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decrypting keystore")
}

// TestPrivateKeyRoundTrip verifies that a key can be saved and loaded correctly
func TestPrivateKeyRoundTrip(t *testing.T) {
	// Generate original key
	originalKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Save as hex file
	keyBytes := crypto.FromECDSA(originalKey)
	keyHex := hex.EncodeToString(keyBytes)
	hexFile := filepath.Join(t.TempDir(), "key.hex")
	err = os.WriteFile(hexFile, []byte(keyHex), 0600)
	require.NoError(t, err)

	// Load from hex file
	loadedKey, err := LoadPrivateKey(hexFile)
	require.NoError(t, err)

	// Verify keys match
	assert.Equal(t, originalKey.D, loadedKey.D)
	assert.Equal(t, originalKey.X, loadedKey.X)
	assert.Equal(t, originalKey.Y, loadedKey.Y)

	// Verify addresses match
	originalAddr := crypto.PubkeyToAddress(originalKey.PublicKey)
	loadedAddr := crypto.PubkeyToAddress(loadedKey.PublicKey)
	assert.Equal(t, originalAddr, loadedAddr)
}

// BenchmarkLoadPrivateKey benchmarks loading a private key from file
func BenchmarkLoadPrivateKey(b *testing.B) {
	privateKey, _ := crypto.GenerateKey()
	keyBytes := crypto.FromECDSA(privateKey)
	keyHex := hex.EncodeToString(keyBytes)
	tempFile := filepath.Join(b.TempDir(), "key.hex")
	os.WriteFile(tempFile, []byte(keyHex), 0600)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadPrivateKey(tempFile)
	}
}

// BenchmarkLoadPrivateKeyFromKeystore benchmarks loading from encrypted keystore
func BenchmarkLoadPrivateKeyFromKeystore(b *testing.B) {
	privateKey, _ := crypto.GenerateKey()
	ks := keystore.NewKeyStore(b.TempDir(), keystore.StandardScryptN, keystore.StandardScryptP)
	account, _ := ks.ImportECDSA(privateKey, "test-password")
	keystorePath := account.URL.Path

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadPrivateKeyFromKeystore(keystorePath, "test-password")
	}
}
