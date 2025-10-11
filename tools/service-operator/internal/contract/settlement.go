package contract

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

const (
	// NETWORK_FEE is the required native token fee for settling rails (0.0013 FIL)
	NETWORK_FEE = 1300000000000000 // 1300000 gwei = 0.0013 FIL
)

// SettlementResult contains the result of settling a payment rail
type SettlementResult struct {
	RailID                  *big.Int
	TotalSettledAmount      *big.Int // Total amount settled and transferred
	TotalNetPayeeAmount     *big.Int // Net amount credited to payee after fees
	TotalOperatorCommission *big.Int // Commission credited to operator
	FinalSettledEpoch       *big.Int // Epoch up to which settlement was completed
	Note                    string   // Additional information from validator
	TransactionHash         common.Hash
}

// RailInfo contains information about a payment rail
type RailInfo struct {
	RailID              *big.Int
	Token               common.Address
	From                common.Address // Payer
	To                  common.Address // Payee
	Operator            common.Address
	Validator           common.Address
	PaymentRate         *big.Int
	LockupPeriod        *big.Int
	LockupFixed         *big.Int
	SettledUpTo         *big.Int
	EndEpoch            *big.Int
	CommissionRateBps   *big.Int
	ServiceFeeRecipient common.Address
	IsTerminated        bool
}

// QueryRailInfo queries information about a specific payment rail
func QueryRailInfo(ctx context.Context, rpcURL string, paymentsAddress common.Address, railID *big.Int) (*RailInfo, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to RPC: %w", err)
	}
	defer client.Close()

	// Create contract instance
	payments, err := bindings.NewPaymentsCaller(paymentsAddress, client)
	if err != nil {
		return nil, fmt.Errorf("creating payments caller: %w", err)
	}

	// Query rail information
	railView, err := payments.GetRail(&bind.CallOpts{Context: ctx}, railID)
	if err != nil {
		return nil, fmt.Errorf("getting rail info: %w", err)
	}

	return &RailInfo{
		RailID:              railID,
		Token:               railView.Token,
		From:                railView.From,
		To:                  railView.To,
		Operator:            railView.Operator,
		Validator:           railView.Validator,
		PaymentRate:         railView.PaymentRate,
		LockupPeriod:        railView.LockupPeriod,
		LockupFixed:         railView.LockupFixed,
		SettledUpTo:         railView.SettledUpTo,
		EndEpoch:            railView.EndEpoch,
		CommissionRateBps:   railView.CommissionRateBps,
		ServiceFeeRecipient: railView.ServiceFeeRecipient,
		IsTerminated:        railView.EndEpoch.Cmp(big.NewInt(0)) > 0,
	}, nil
}

// QueryRailsForPayee queries all payment rails for a specific payee and token
func QueryRailsForPayee(ctx context.Context, rpcURL string, paymentsAddress common.Address, payee common.Address, token common.Address) ([]bindings.PaymentsRailInfo, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to RPC: %w", err)
	}
	defer client.Close()

	// Create contract instance
	payments, err := bindings.NewPaymentsCaller(paymentsAddress, client)
	if err != nil {
		return nil, fmt.Errorf("creating payments caller: %w", err)
	}

	// Query rails for payee
	rails, err := payments.GetRailsForPayeeAndToken(&bind.CallOpts{Context: ctx}, payee, token)
	if err != nil {
		return nil, fmt.Errorf("getting rails for payee: %w", err)
	}

	return rails, nil
}

// SettleRail settles a payment rail up to the specified epoch
// This function sends a transaction to settle the rail and waits for it to be mined
func SettleRail(ctx context.Context, rpcURL string, paymentsAddress common.Address, auth *bind.TransactOpts, railID *big.Int, untilEpoch *big.Int) (*SettlementResult, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to RPC: %w", err)
	}
	defer client.Close()

	// Create contract instance
	payments, err := bindings.NewPaymentsTransactor(paymentsAddress, client)
	if err != nil {
		return nil, fmt.Errorf("creating payments transactor: %w", err)
	}

	// Set the NETWORK_FEE as the value to send with the transaction
	auth.Value = big.NewInt(NETWORK_FEE)

	// Send settlement transaction
	tx, err := payments.SettleRail(auth, railID, untilEpoch)
	if err != nil {
		return nil, fmt.Errorf("sending settle transaction: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return nil, fmt.Errorf("waiting for transaction to be mined: %w", err)
	}

	if receipt.Status != 1 {
		return nil, fmt.Errorf("transaction failed with status %d", receipt.Status)
	}

	// Parse the RailSettled event from the receipt to get settlement details
	paymentsFilterer, err := bindings.NewPaymentsFilterer(paymentsAddress, client)
	if err != nil {
		return nil, fmt.Errorf("creating payments filterer: %w", err)
	}

	// Look for RailSettled event in the logs
	var settlementResult *SettlementResult
	for _, log := range receipt.Logs {
		event, err := paymentsFilterer.ParseRailSettled(*log)
		if err != nil {
			// Not a RailSettled event, skip
			continue
		}

		// Found the settlement event
		settlementResult = &SettlementResult{
			RailID:                  railID,
			TotalSettledAmount:      event.TotalSettledAmount,
			TotalNetPayeeAmount:     event.TotalNetPayeeAmount,
			TotalOperatorCommission: event.OperatorCommission,
			FinalSettledEpoch:       event.SettledUpTo,
			Note:                    "", // Note is not in the event, would need to decode from tx output
			TransactionHash:         tx.Hash(),
		}
		break
	}

	if settlementResult == nil {
		// No RailSettled event found - this might mean the rail was already settled
		// or the settlement didn't progress. Return basic info.
		settlementResult = &SettlementResult{
			RailID:                  railID,
			TotalSettledAmount:      big.NewInt(0),
			TotalNetPayeeAmount:     big.NewInt(0),
			TotalOperatorCommission: big.NewInt(0),
			FinalSettledEpoch:       big.NewInt(0),
			Note:                    "Settlement transaction succeeded but no RailSettled event found (rail may already be settled)",
			TransactionHash:         tx.Hash(),
		}
	}

	return settlementResult, nil
}
