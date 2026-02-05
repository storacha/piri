package testutil

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/storacha/filecoin-services/go/bindings"

	"github.com/storacha/piri/pkg/testutil/localdev"
)

// Operator assumes the role of a contract operator, i.e. Storacha.
// The operator can be used to mutate contract state as a contract owner (e.g. approve a provider) and
// as a payer.
type Operator struct {
	t testing.TB

	chain     *AnvilClient
	contracts localdev.ContractAddresses
	accounts  localdev.ChainAccounts

	ethClient *ethclient.Client

	deployerKey *ecdsa.PrivateKey
	payerKey    *ecdsa.PrivateKey
}

func NewOperator(
	t testing.TB,
	ethClient *ethclient.Client,
	chain *AnvilClient,
	contracts localdev.ContractAddresses,
	accounts localdev.ChainAccounts,
) *Operator {
	return &Operator{
		t:           t,
		chain:       chain,
		contracts:   contracts,
		accounts:    accounts,
		ethClient:   ethClient,
		deployerKey: HexToECDSA(t, accounts.Deployer.PrivateKey),
		payerKey:    HexToECDSA(t, accounts.Payer.PrivateKey),
	}
}

func (o *Operator) ApproveProvider(id uint64) {
	// Create transactor with Deployer's private key
	auth, err := bind.NewKeyedTransactorWithChainID(o.deployerKey, big.NewInt(localdev.ChainID))
	if err != nil {
		o.t.Fatal(err)
	}

	// Get FilecoinWarmStorageService contract binding using container addresses
	serviceContract, err := bindings.NewFilecoinWarmStorageService(
		common.HexToAddress(o.contracts.FilecoinWarmStorageService),
		o.ethClient,
	)
	if err != nil {
		o.t.Fatal(err)
	}

	// Call AddApprovedProvider
	tx, err := serviceContract.AddApprovedProvider(auth, big.NewInt(int64(id)))
	if err != nil {
		o.t.Fatal(err)
	}

	o.WaitForTxConfirmation(tx.Hash(), 30*time.Second)
}

// WaitForTxConfirmation waits for a transaction to be mined and confirmed.
// It actively mines blocks to help the transaction get included.
func (o *Operator) WaitForTxConfirmation(txHash common.Hash, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(o.t.Context(), timeout)
	defer cancel()

	for {
		receipt, err := o.ethClient.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			if receipt.Status == ethtypes.ReceiptStatusFailed {
				o.t.Fatal(fmt.Errorf("transaction %s failed", txHash.Hex()))
			}
			return
		}

		// Mine a block to help confirmation
		_ = o.chain.MineBlock()

		select {
		case <-ctx.Done():
			o.t.Fatal(fmt.Errorf("timeout waiting for tx %s: %w", txHash.Hex(), ctx.Err()))
		case <-time.After(200 * time.Millisecond):
		}
	}
}
