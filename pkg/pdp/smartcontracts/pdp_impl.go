package smartcontracts

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

// PDPContract is the concrete implementation of the PDP interface
type PDPContract struct{}

// Ensure PDPContract implements PDP interface
var _ PDP = (*PDPContract)(nil)

// NewPDPContract creates a new instance of PDPContract
func NewPDPContract() PDP {
	return &PDPContract{}
}

// NewPDPVerifier creates a new PDPVerifier contract instance
func (p *PDPContract) NewPDPVerifier(address common.Address, backend bind.ContractBackend) (PDPVerifier, error) {
	return bindings.NewPDPVerifier(address, backend)
}

// NewPDPProvingSchedule creates a new PDPProvingSchedule contract instance
func (p *PDPContract) NewPDPProvingSchedule(address common.Address, backend bind.ContractBackend) (PDPProvingSchedule, error) {
	return bindings.NewPDPProvingSchedule(address, backend)
}

// NewFilecoinWarmStorageService creates a new FilecoinWarmStorageService contract instance
func (p *PDPContract) NewFilecoinWarmStorageService(address common.Address, backend bind.ContractBackend) (FilecoinWarmStorageService, error) {
	return bindings.NewFilecoinWarmStorageService(address, backend)
}

// NewServiceProviderRegistry creates a new ServiceProviderRegistry contract instance
func (p *PDPContract) NewServiceProviderRegistry(address common.Address, backend bind.ContractBackend) (ServiceProviderRegistry, error) {
	return bindings.NewServiceProviderRegistry(address, backend)
}

// GetDataSetIdFromReceipt parses DataSetCreated event from transaction receipt
func (p *PDPContract) GetDataSetIdFromReceipt(receipt *types.Receipt) (uint64, error) {
	pdpABI, err := bindings.PDPVerifierMetaData.GetAbi()
	if err != nil {
		return 0, fmt.Errorf("failed to get PDP ABI: %w", err)
	}

	event, exists := pdpABI.Events["DataSetCreated"]
	if !exists {
		return 0, fmt.Errorf("DataSetCreated event not found in ABI")
	}

	for _, vLog := range receipt.Logs {
		if len(vLog.Topics) > 0 && vLog.Topics[0] == event.ID {
			if len(vLog.Topics) < 2 {
				return 0, fmt.Errorf("log does not contain setId topic")
			}
			setIdBigInt := new(big.Int).SetBytes(vLog.Topics[1].Bytes())
			return setIdBigInt.Uint64(), nil
		}
	}

	return 0, fmt.Errorf("DataSetCreated event not found in receipt")
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
