package smartcontracts

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

// Main factory interface for creating contract instances and parsing events
type PDP interface {
	// Factory methods for contract instances
	NewPDPVerifier(address common.Address, backend bind.ContractBackend) (PDPVerifier, error)
	NewPDPProvingSchedule(address common.Address, backend bind.ContractBackend) (PDPProvingSchedule, error)

	// Event parsing helpers
	GetDataSetIdFromReceipt(receipt *types.Receipt) (uint64, error)
	GetPieceIdsFromReceipt(receipt *types.Receipt) ([]uint64, error)
}

// PDPProvingSchedule interface for managing challenge windows
type PDPProvingSchedule interface {
	// GetPDPConfig returns all PDP configuration parameters
	GetPDPConfig(opts *bind.CallOpts) (struct {
		MaxProvingPeriod         uint64
		ChallengeWindow          *big.Int
		ChallengesPerProof       *big.Int
		InitChallengeWindowStart *big.Int
	}, error)
	NextPDPChallengeWindowStart(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
}

// PDPVerifier interface matching the IPDPVerifier.sol contract
type PDPVerifier interface {
	// View functions
	GetChallengeFinality(opts *bind.CallOpts) (*big.Int, error)
	GetDataSetLeafCount(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	GetNextChallengeEpoch(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	GetDataSetListener(opts *bind.CallOpts, setId *big.Int) (common.Address, error)
	GetDataSetStorageProvider(opts *bind.CallOpts, setId *big.Int) (common.Address, common.Address, error)
	GetChallengeRange(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	GetScheduledRemovals(opts *bind.CallOpts, setId *big.Int) ([]*big.Int, error)
	FindPieceIds(opts *bind.CallOpts, setId *big.Int, leafIndexs []*big.Int) ([]bindings.IPDPTypesPieceIdAndOffset, error)
	GetNextPieceId(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)

	// CalculateProofFee returns the required proof fee based on the dataset size and estimated gas fee
	CalculateProofFee(opts *bind.CallOpts, setId *big.Int, estimatedGasFee *big.Int) (*big.Int, error)
}
