package contract

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

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

// CreateTransactor creates transaction auth with automatic gas estimation
func CreateTransactor(ctx context.Context, client *ethclient.Client, privateKey *ecdsa.PrivateKey) (*bind.TransactOpts, error) {
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting chain ID: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("creating transactor: %w", err)
	}

	// Let the client estimate gas
	auth.GasLimit = 0

	return auth, nil
}

// WaitForTransaction waits for a transaction to be mined and returns the receipt
// Uses exponential backoff with a timeout of 5 Filecoin epochs (150 seconds)
func WaitForTransaction(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	fmt.Printf("Transaction submitted: %s\n", txHash.Hex())
	fmt.Println("Waiting for confirmation...")

	const (
		filecoinEpochDuration = 30 * time.Second
		maxEpochs             = 5
		maxElapsedTime        = filecoinEpochDuration * maxEpochs // 150 seconds
	)

	// Configure exponential backoff
	// Start with 5 seconds, max 30 seconds (one epoch), with 2x multiplier
	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.InitialInterval = 5 * time.Second
	exponentialBackoff.MaxInterval = filecoinEpochDuration
	exponentialBackoff.Multiplier = 2.0

	operation := func() (*types.Receipt, error) {
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err != nil {
			// Transaction not yet mined, retry
			return nil, err
		}

		if receipt.Status != types.ReceiptStatusSuccessful {
			// Transaction failed, don't retry
			return nil, backoff.Permanent(fmt.Errorf("transaction failed with status %d", receipt.Status))
		}

		// Success
		return receipt, nil
	}

	// Use backoff.Retry with context and options
	receipt, err := backoff.Retry(
		ctx,
		operation,
		backoff.WithBackOff(exponentialBackoff),
		backoff.WithMaxElapsedTime(maxElapsedTime),
		backoff.WithNotify(func(err error, duration time.Duration) {
			fmt.Printf("Transaction not yet confirmed, retrying in %v...\n", duration)
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("waiting for transaction: %w", err)
	}

	return receipt, nil
}

// GetProviderApprovedEvent parses the ProviderApproved event from a transaction receipt
func GetProviderApprovedEvent(receipt *types.Receipt) (*big.Int, error) {
	// Event signature: ProviderApproved(uint256 indexed providerId)
	eventSignature := crypto.Keccak256Hash([]byte("ProviderApproved(uint256)"))

	for _, log := range receipt.Logs {
		if len(log.Topics) > 0 && log.Topics[0] == eventSignature {
			if len(log.Topics) >= 2 {
				providerId := new(big.Int).SetBytes(log.Topics[1].Bytes())
				return providerId, nil
			}
		}
	}

	return nil, fmt.Errorf("ProviderApproved event not found in receipt")
}
