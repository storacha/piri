package smartcontracts

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

// ViewContractHelper provides helper functions for interacting with FilecoinWarmStorageServiceStateView
type ViewContractHelper struct {
	viewContract *bindings.FilecoinWarmStorageServiceStateView
	client       *ethclient.Client
}

// NewViewContractHelper creates a new view contract helper
// It first gets the view contract address from the main service contract, then connects to it
func NewViewContractHelper(client *ethclient.Client, serviceContractAddress common.Address) (*ViewContractHelper, error) {
	// Get the main service contract
	serviceContract, err := bindings.NewFilecoinWarmStorageService(serviceContractAddress, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind service contract: %w", err)
	}

	// Get the view contract address from the service contract
	viewAddress, err := serviceContract.ViewContractAddress(&bind.CallOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get view contract address: %w", err)
	}

	// Check if view contract address is set
	if viewAddress == (common.Address{}) {
		return nil, fmt.Errorf("view contract not set on service contract at %s", serviceContractAddress.Hex())
	}

	// Connect to the view contract
	viewContract, err := bindings.NewFilecoinWarmStorageServiceStateView(viewAddress, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind view contract at %s: %w", viewAddress.Hex(), err)
	}

	return &ViewContractHelper{
		viewContract: viewContract,
		client:       client,
	}, nil
}

// GetNextClientDataSetId returns the next client dataset ID that will be assigned
// This is the value that needs to be signed for CreateDataSet operations
func (v *ViewContractHelper) GetNextClientDataSetId(payerAddress common.Address) (*big.Int, error) {
	// The view contract exposes clientDataSetIDs mapping as a public getter
	nextId, err := v.viewContract.ClientDataSetIDs(&bind.CallOpts{}, payerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get next client dataset ID for %s: %w", payerAddress.Hex(), err)
	}
	return nextId, nil
}

// DataSetInfo holds information about a dataset from the view contract
type DataSetInfo struct {
	PdpRailId       *big.Int
	CacheMissRailId *big.Int
	CdnRailId       *big.Int
	Payer           common.Address
	Payee           common.Address
	ServiceProvider common.Address
	CommissionBps   *big.Int
	ClientDataSetId *big.Int
	PdpEndEpoch     *big.Int
	ProviderId      *big.Int
	DataSetId       *big.Int
}

// GetDataSet retrieves information about a specific dataset
func (v *ViewContractHelper) GetDataSet(dataSetId *big.Int) (*DataSetInfo, error) {
	result, err := v.viewContract.GetDataSet(&bind.CallOpts{}, dataSetId)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset %s: %w", dataSetId.String(), err)
	}

	// Convert the result to our DataSetInfo struct
	// The result is a struct with these fields based on the contract
	info := &DataSetInfo{
		PdpRailId:       result.PdpRailId,
		CacheMissRailId: result.CacheMissRailId,
		CdnRailId:       result.CdnRailId,
		Payer:           result.Payer,
		Payee:           result.Payee,
		ServiceProvider: result.ServiceProvider,
		CommissionBps:   result.CommissionBps,
		ClientDataSetId: result.ClientDataSetId,
		PdpEndEpoch:     result.PdpEndEpoch,
		ProviderId:      result.ProviderId,
		DataSetId:       result.DataSetId,
	}

	// Check if dataset exists (pdpRailId would be 0 if not)
	if info.PdpRailId.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("dataset %s does not exist", dataSetId.String())
	}

	return info, nil
}

// GetClientDataSets returns all dataset IDs for a given client/payer
func (v *ViewContractHelper) GetClientDataSets(clientAddress common.Address) ([]*big.Int, error) {
	dataSetIds, err := v.viewContract.ClientDataSets(&bind.CallOpts{}, clientAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get client datasets for %s: %w", clientAddress.Hex(), err)
	}
	return dataSetIds, nil
}

// GetDataSetMetadata retrieves all metadata for a dataset
func (v *ViewContractHelper) GetDataSetMetadata(dataSetId *big.Int) (map[string]string, error) {
	result, err := v.viewContract.GetAllDataSetMetadata(&bind.CallOpts{}, dataSetId)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset metadata for %s: %w", dataSetId.String(), err)
	}

	// result is a struct with Keys and Values arrays
	metadata := make(map[string]string)
	for i := range result.Keys {
		metadata[result.Keys[i]] = result.Values[i]
	}

	return metadata, nil
}

// GetPieceMetadata retrieves all metadata for a specific piece in a dataset
func (v *ViewContractHelper) GetPieceMetadata(dataSetId, pieceId *big.Int) (map[string]string, error) {
	result, err := v.viewContract.GetAllPieceMetadata(&bind.CallOpts{}, dataSetId, pieceId)
	if err != nil {
		return nil, fmt.Errorf("failed to get piece metadata for dataset %s piece %s: %w",
			dataSetId.String(), pieceId.String(), err)
	}

	// result is a struct with Keys and Values arrays
	metadata := make(map[string]string)
	for i := range result.Keys {
		metadata[result.Keys[i]] = result.Values[i]
	}

	return metadata, nil
}

// IsProviderApproved checks if a provider ID is approved
func (v *ViewContractHelper) IsProviderApproved(providerId *big.Int) (bool, error) {
	approved, err := v.viewContract.IsProviderApproved(&bind.CallOpts{}, providerId)
	if err != nil {
		return false, fmt.Errorf("failed to check if provider %s is approved: %w", providerId.String(), err)
	}
	return approved, nil
}

// GetApprovedProviders returns list of approved provider IDs with pagination
// offset: starting index (0-based)
// limit: maximum number of results to return
func (v *ViewContractHelper) GetApprovedProviders(offset, limit *big.Int) ([]*big.Int, error) {
	providerIds, err := v.viewContract.GetApprovedProviders(&bind.CallOpts{}, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get approved providers: %w", err)
	}
	return providerIds, nil
}

// GetAllApprovedProviders returns all approved provider IDs (convenience function)
func (v *ViewContractHelper) GetAllApprovedProviders() ([]*big.Int, error) {
	// Get the total count first
	count, err := v.viewContract.GetApprovedProvidersLength(&bind.CallOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get approved providers count: %w", err)
	}

	// Get all providers in one call
	return v.GetApprovedProviders(big.NewInt(0), count)
}

// GetMaxProvingPeriod returns the maximum proving period in epochs
func (v *ViewContractHelper) GetMaxProvingPeriod() (*big.Int, error) {
	maxPeriod, err := v.viewContract.GetMaxProvingPeriod(&bind.CallOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get max proving period: %w", err)
	}
	// Convert uint64 to *big.Int
	return new(big.Int).SetUint64(maxPeriod), nil
}

// GetChallengeWindow returns the challenge window size in epochs
func (v *ViewContractHelper) GetChallengeWindow() (*big.Int, error) {
	window, err := v.viewContract.ChallengeWindow(&bind.CallOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge window: %w", err)
	}
	return window, nil
}