package smartcontracts

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/xerrors"

	internal2 "github.com/storacha/piri/pkg/pdp/contract/internal"
)

type PDP interface {
	NewPDPVerifier(address common.Address, backend bind.ContractBackend) (PDPVerifier, error)
	NewIPDPProvingSchedule(address common.Address, backend bind.ContractBackend) (PDPProvingSchedule, error)
	GetProofSetIdFromReceipt(receipt *types.Receipt) (uint64, error)
	GetRootIdsFromReceipt(receipt *types.Receipt) ([]uint64, error)
}

type PDPProvingSchedule interface {
	InitChallengeWindowStart(opts *bind.CallOpts) (*big.Int, error)
	NextChallengeWindowStart(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	GetMaxProvingPeriod(opts *bind.CallOpts) (uint64, error)
	ChallengeWindow(opts *bind.CallOpts) (*big.Int, error)
}

type PDPVerifier interface {
	GetProofSetListener(opts *bind.CallOpts, setId *big.Int) (common.Address, error)
	GetProofSetOwner(opts *bind.CallOpts, setId *big.Int) (common.Address, common.Address, error)
	GetNextChallengeEpoch(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	GetChallengeRange(opts *bind.CallOpts, setId *big.Int) (*big.Int, error)
	FindRootIds(opts *bind.CallOpts, setId *big.Int, leafIndexs []*big.Int) ([]internal2.PDPVerifierRootIdAndOffset, error)
	GetScheduledRemovals(opts *bind.CallOpts, setId *big.Int) ([]*big.Int, error)
	CalculateProofFee(opts *bind.CallOpts, setId *big.Int, estimatedGasFee *big.Int) (*big.Int, error)
	GetChallengeFinality(opts *bind.CallOpts) (*big.Int, error)
}

func PDPVerifierMetaData() (*abi.ABI, error) {
	return internal2.PDPVerifierMetaData.GetAbi()
}

type PDPVerifierProof = internal2.PDPVerifierProof

type PDPContract struct{}

var _ PDP = (*PDPContract)(nil)

func (p *PDPContract) NewPDPVerifier(address common.Address, backend bind.ContractBackend) (PDPVerifier, error) {
	return internal2.NewPDPVerifier(address, backend)
}

func (p *PDPContract) NewIPDPProvingSchedule(address common.Address, backend bind.ContractBackend) (PDPProvingSchedule, error) {
	return internal2.NewIPDPProvingSchedule(address, backend)
}

func (p *PDPContract) GetProofSetIdFromReceipt(receipt *types.Receipt) (uint64, error) {
	pdpABI, err := PDPVerifierMetaData()
	if err != nil {
		return 0, xerrors.Errorf("failed to get PDP ABI: %w", err)
	}

	event, exists := pdpABI.Events["ProofSetCreated"]
	if !exists {
		return 0, xerrors.Errorf("ProofSetCreated event not found in ABI")
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

	return 0, xerrors.Errorf("ProofSetCreated event not found in receipt")
}

func (p *PDPContract) GetRootIdsFromReceipt(receipt *types.Receipt) ([]uint64, error) {
	// Get the ABI from the contract metadata
	pdpABI, err := PDPVerifierMetaData()
	if err != nil {
		return nil, fmt.Errorf("failed to get PDP ABI: %w", err)
	}

	// Get the event definition
	event, exists := pdpABI.Events["RootsAdded"]
	if !exists {
		return nil, fmt.Errorf("RootsAdded event not found in ABI")
	}

	var rootIds []uint64
	eventFound := false

	// Iterate over the logs in the receipt
	for _, vLog := range receipt.Logs {
		// Check if the log corresponds to the RootsAdded event
		if len(vLog.Topics) > 0 && vLog.Topics[0] == event.ID {
			// The setId is an indexed parameter in Topics[1], but we don't need it here
			// as we already have the proofset ID from the database

			// Parse the non-indexed parameter (rootIds array) from the data
			unpacked, err := event.Inputs.Unpack(vLog.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to unpack log data: %w", err)
			}

			// Extract the rootIds array
			if len(unpacked) == 0 {
				return nil, fmt.Errorf("no unpacked data found in log")
			}

			// Convert the unpacked rootIds ([]interface{} containing *big.Int) to []uint64
			bigIntRootIds, ok := unpacked[0].([]*big.Int)
			if !ok {
				return nil, fmt.Errorf("failed to convert unpacked data to array")
			}

			rootIds = make([]uint64, len(bigIntRootIds))
			for i := range bigIntRootIds {
				rootIds[i] = bigIntRootIds[i].Uint64()
			}

			eventFound = true
			// We found the event, so we can break the loop
			break
		}
	}

	if !eventFound {
		return nil, fmt.Errorf("RootsAdded event not found in receipt")
	}

	return rootIds, nil
}
