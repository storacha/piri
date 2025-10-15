package smartcontracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	chaintypes "github.com/filecoin-project/lotus/chain/types"
	"golang.org/x/xerrors"

	"github.com/storacha/filecoin-services/go/bindings"
)

// ChainAPI interface for chain operations needed by proving schedule
type ChainAPI interface {
	ChainHead(context.Context) (*chaintypes.TipSet, error)
}

// PDP is the main factory interface for creating contract instances and parsing
type PDP interface {
	// NewPDPVerifier is a factory methods for contract instances
	NewPDPVerifier(address common.Address, backend bind.ContractBackend) (PDPVerifier, error)

	// GetDataSetIdFromReceipt parses a data set id from a receipt
	GetDataSetIdFromReceipt(receipt *types.Receipt) (uint64, error)
	// GetPieceIdsFromReceipt parses a data set id from a receipt
	GetPieceIdsFromReceipt(receipt *types.Receipt) ([]uint64, error)
}

// PDPVerifier interface matching the IPDPVerifier.sol contract
type PDPVerifier interface {
	GetChallengeFinality(opts *bind.CallOpts) (*big.Int, error)
	GetDataSetLeafCount(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	GetNextChallengeEpoch(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	GetDataSetListener(opts *bind.CallOpts, setId *big.Int) (common.Address, error)
	GetDataSetStorageProvider(opts *bind.CallOpts, setId *big.Int) (common.Address, common.Address, error)
	GetChallengeRange(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	GetScheduledRemovals(opts *bind.CallOpts, setId *big.Int) ([]*big.Int, error)
	FindPieceIds(opts *bind.CallOpts, setId *big.Int, leafIndexs []*big.Int) ([]bindings.IPDPTypesPieceIdAndOffset, error)
	GetNextPieceId(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	CalculateProofFee(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
}

// ProvingScheduleProvider abstracts proving schedule operations.
// This can be backed by hardcoded values or a real service contract.
type ProvingScheduleProvider interface {
	// GetPDPConfig returns the proving configuration parameters
	GetPDPConfig(ctx context.Context) (PDPConfig, error)

	// NextPDPChallengeWindowStart calculates the next challenge window start epoch
	NextPDPChallengeWindowStart(ctx context.Context, setId *big.Int) (*big.Int, error)
}

// PDPConfig holds proving period configuration
type PDPConfig struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}

// PDPContract is the concrete implementation of the PDP interface
type PDPContract struct{}

// Ensure PDPContract implements PDP interface
var _ PDP = (*PDPContract)(nil)

// NewPDPVerifier creates a new PDPVerifier contract instance
func (p *PDPContract) NewPDPVerifier(address common.Address, backend bind.ContractBackend) (PDPVerifier, error) {
	return bindings.NewPDPVerifier(address, backend)
}

// GetDataSetIdFromReceipt parses DataSetCreated event from transaction receipt
func (p *PDPContract) GetDataSetIdFromReceipt(receipt *types.Receipt) (uint64, error) {
	pdpABI, err := bindings.PDPVerifierMetaData.GetAbi()
	if err != nil {
		return 0, xerrors.Errorf("failed to get PDP ABI: %w", err)
	}

	event, exists := pdpABI.Events["DataSetCreated"]
	if !exists {
		return 0, xerrors.Errorf("DataSetCreated event not found in ABI")
	}

	for _, vLog := range receipt.Logs {
		if len(vLog.Topics) > 0 && vLog.Topics[0] == event.ID {
			if len(vLog.Topics) < 2 {
				return 0, xerrors.Errorf("log does not contain setId topic")
			}
			setIdBigInt := new(big.Int).SetBytes(vLog.Topics[1].Bytes())
			return setIdBigInt.Uint64(), nil
		}
	}

	return 0, xerrors.Errorf("DataSetCreated event not found in receipt")
}

// GetPieceIdsFromReceipt parses PiecesAdded event from transaction receipt
func (p *PDPContract) GetPieceIdsFromReceipt(receipt *types.Receipt) ([]uint64, error) {
	pdpABI, err := bindings.PDPVerifierMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get PDP ABI: %w", err)
	}

	event, exists := pdpABI.Events["PiecesAdded"]
	if !exists {
		return nil, fmt.Errorf("PiecesAdded event not found in ABI")
	}

	eventFound := false
	for _, vLog := range receipt.Logs {
		if len(vLog.Topics) > 0 && vLog.Topics[0] == event.ID {
			// The setId is an indexed parameter in Topics[1]
			// The pieceIds array is in the data field

			unpacked, err := event.Inputs.NonIndexed().Unpack(vLog.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to unpack log data: %w", err)
			}

			if len(unpacked) < 1 {
				return nil, fmt.Errorf("no unpacked data found in log")
			}

			// The first non-indexed parameter is pieceIds
			bigIntPieceIds, ok := unpacked[0].([]*big.Int)
			if !ok {
				return nil, fmt.Errorf("failed to convert unpacked data to []*big.Int array")
			}

			pieceIds := make([]uint64, len(bigIntPieceIds))
			for i := range bigIntPieceIds {
				pieceIds[i] = bigIntPieceIds[i].Uint64()
			}

			eventFound = true
			return pieceIds, nil
		}
	}

	if !eventFound {
		return nil, fmt.Errorf("PiecesAdded event not found in receipt")
	}

	return nil, fmt.Errorf("unexpected error in GetPieceIdsFromReceipt")
}

// PDPVerifierMetaData returns the ABI for the PDPVerifier contract
func PDPVerifierMetaData() (*abi.ABI, error) {
	return bindings.PDPVerifierMetaData.GetAbi()
}
