package config

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	NetworkCalibration = "calibration"
	NetworkMainnet     = "mainnet"
)

const (
	DefaultRpcUrl          = "https://api.calibration.node.glif.io/rpc/v1"
	DefaultContractAddress = "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91"
	DefaultNetwork         = NetworkCalibration
)

type Config struct {
	RPCUrl           string
	ContractAddress  string
	PrivateKeyPath   string
	KeystorePath     string
	KeystorePassword string
	Network          string
}

// NetworkDefaults returns default RPC URL and contract address for a given network
func NetworkDefaults(network string) (rpcURL string, contractAddr string, err error) {
	switch network {
	case NetworkCalibration:
		return DefaultRpcUrl, DefaultContractAddress, nil
	case NetworkMainnet:
		return "", "", fmt.Errorf("mainnet network not yet supported")
	default:
		return "", "", fmt.Errorf("unknown network: %s (supported: calibration, mainnet)", network)
	}
}

// Validate checks that required configuration fields are set
func (c *Config) Validate() error {
	if c.RPCUrl == "" {
		return fmt.Errorf("rpc-url is required")
	}

	if c.ContractAddress == "" {
		return fmt.Errorf("contract-address is required")
	}

	if !common.IsHexAddress(c.ContractAddress) {
		return fmt.Errorf("invalid contract address: %s", c.ContractAddress)
	}

	if c.PrivateKeyPath == "" && c.KeystorePath == "" {
		return fmt.Errorf("either private-key or keystore must be provided")
	}

	if c.KeystorePath != "" && c.KeystorePassword == "" {
		return fmt.Errorf("keystore-password is required when using keystore")
	}

	return nil
}

// ContractAddr returns the contract address as a common.Address
func (c *Config) ContractAddr() common.Address {
	return common.HexToAddress(c.ContractAddress)
}

// LoadPrivateKey loads a private key from a file
// The file can contain either hex-encoded or raw bytes
func LoadPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading private key file: %w", err)
	}

	// Trim whitespace
	keyData := strings.TrimSpace(string(data))

	// Try hex decoding first
	if strings.HasPrefix(keyData, "0x") {
		keyData = keyData[2:]
	}

	keyBytes, err := hex.DecodeString(keyData)
	if err != nil {
		// If hex decoding fails, try using the raw bytes
		keyBytes = data
	}

	privateKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	return privateKey, nil
}

// LoadPrivateKeyFromKeystore loads a private key from an encrypted keystore file
func LoadPrivateKeyFromKeystore(keystorePath, password string) (*ecdsa.PrivateKey, error) {
	keystoreJSON, err := os.ReadFile(keystorePath)
	if err != nil {
		return nil, fmt.Errorf("reading keystore file: %w", err)
	}

	key, err := keystore.DecryptKey(keystoreJSON, password)
	if err != nil {
		return nil, fmt.Errorf("decrypting keystore: %w", err)
	}

	return key.PrivateKey, nil
}
