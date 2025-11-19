package smartcontracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/filecoin-services/go/bindings"
)

type Service interface {
	PDPConfig(ctx context.Context) (PDPConfig, error)
	GetNextClientDataSetId(ctx context.Context, payerAddress common.Address) (*big.Int, error)
	GetDataSet(ctx context.Context, dataSetId *big.Int) (*DataSetInfo, error)
	GetClientDataSets(ctx context.Context, clientAddress common.Address) ([]*big.Int, error)
	GetDataSetMetadata(ctx context.Context, dataSetId *big.Int) (map[string]string, error)
	GetPieceMetadata(ctx context.Context, dataSetId, pieceId *big.Int) (map[string]string, error)
	IsProviderApproved(ctx context.Context, providerId *big.Int) (bool, error)
	NextPDPChallengeWindowStart(ctx context.Context, proofSetID *big.Int) (*big.Int, error)
	GetApprovedProviders(ctx context.Context, offset, limit *big.Int) ([]*big.Int, error)
	GetAllApprovedProviders(ctx context.Context) ([]*big.Int, error)
	GetMaxProvingPeriod(ctx context.Context) (*big.Int, error)
	GetChallengeWindow(ctx context.Context) (*big.Int, error)

	// not part of contract code, added for convience in testing and usage
	Address() common.Address
}

// serviceContract provides helper functions for interacting with FilecoinWarmStorageServiceStateView
type serviceContract struct {
	address      common.Address
	viewContract *bindings.FilecoinWarmStorageServiceStateView
	client       bind.ContractBackend
}

// NewServiceView creates a new view contract helper
// It first gets the view contract address from the main service contract, then connects to it
func NewServiceView(address common.Address, client bind.ContractBackend) (Service, error) {
	// Get the main service contract
	sc, err := bindings.NewFilecoinWarmStorageService(address, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind service contract: %w", err)
	}

	// Get the view contract address from the service contract
	viewAddress, err := sc.ViewContractAddress(&bind.CallOpts{Context: context.TODO()})
	if err != nil {
		return nil, fmt.Errorf("failed to get view contract address: %w", err)
	}

	// Check if view contract address is set
	if viewAddress == (common.Address{}) {
		return nil, fmt.Errorf("view contract not set on service contract at %s", address.Hex())
	}

	// Connect to the view contract
	viewContract, err := bindings.NewFilecoinWarmStorageServiceStateView(viewAddress, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind view contract at %s: %w", viewAddress.Hex(), err)
	}

	return &serviceContract{
		address:      viewAddress,
		viewContract: viewContract,
		client:       client,
	}, nil
}

// PDPConfig holds proving period configuration
type PDPConfig struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}

func (v *serviceContract) PDPConfig(ctx context.Context) (PDPConfig, error) {
	pdpConfig, err := v.viewContract.GetPDPConfig(&bind.CallOpts{Context: ctx})
	if err != nil {
		return PDPConfig{}, fmt.Errorf("failed to get pdp config: %w", err)
	}

	return PDPConfig{
		MaxProvingPeriod:         pdpConfig.MaxProvingPeriod,
		ChallengeWindow:          pdpConfig.ChallengeWindowSize,
		ChallengesPerProof:       pdpConfig.ChallengesPerProof,
		InitChallengeWindowStart: pdpConfig.InitChallengeWindowStart,
	}, nil
}

// GetNextClientDataSetId returns the next client dataset ID that will be assigned
// This is the value that needs to be signed for CreateDataSet operations
// TODO this is a "dumb" implementation - see PR #265 in FilOzone/filecoin-services repo for context
func (v *serviceContract) GetNextClientDataSetId(ctx context.Context, payerAddress common.Address) (*big.Int, error) {
	// Get all datasets for this payer
	// NOTE: This is inefficient - it fetches ALL datasets for the payer and then iterates through
	// each one individually to find the highest ID.
	//
	// Context: As of PR #265 (commit dc2c8ab), the contract switched from sequential to
	// non-sequential client dataset IDs to enable concurrent dataset creation without conflicts.
	// Clients can now choose any unused ID, rather than getting the next sequential number.
	datasets, err := v.GetClientDataSets(ctx, payerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get client datasets for %s: %w", payerAddress.Hex(), err)
	}

	// If no datasets exist, start with ID 1
	if len(datasets) == 0 {
		return big.NewInt(1), nil
	}

	// Find the highest clientDataSetId
	// This is O(n) where n is the number of datasets, and each GetDataSet call is a separate RPC call
	var maxId *big.Int = big.NewInt(0)
	for _, dataSetId := range datasets {
		info, err := v.GetDataSet(ctx, dataSetId)
		if err != nil {
			continue // Skip datasets we can't read
		}
		if info.ClientDataSetId != nil && info.ClientDataSetId.Cmp(maxId) > 0 {
			maxId = info.ClientDataSetId
		}
	}

	// Return the next ID (max + 1)
	// Note: This is just a suggestion - clients can use any unused ID they prefer
	nextId := new(big.Int).Add(maxId, big.NewInt(1))
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
func (v *serviceContract) GetDataSet(ctx context.Context, dataSetId *big.Int) (*DataSetInfo, error) {
	result, err := v.viewContract.GetDataSet(&bind.CallOpts{Context: ctx}, dataSetId)
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
func (v *serviceContract) GetClientDataSets(ctx context.Context, clientAddress common.Address) ([]*big.Int, error) {
	dataSetIds, err := v.viewContract.ClientDataSets(&bind.CallOpts{Context: ctx}, clientAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get client datasets for %s: %w", clientAddress.Hex(), err)
	}
	return dataSetIds, nil
}

// GetDataSetMetadata retrieves all metadata for a dataset
func (v *serviceContract) GetDataSetMetadata(ctx context.Context, dataSetId *big.Int) (map[string]string, error) {
	result, err := v.viewContract.GetAllDataSetMetadata(&bind.CallOpts{Context: ctx}, dataSetId)
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
func (v *serviceContract) GetPieceMetadata(ctx context.Context, dataSetId, pieceId *big.Int) (map[string]string, error) {
	result, err := v.viewContract.GetAllPieceMetadata(&bind.CallOpts{Context: ctx}, dataSetId, pieceId)
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
func (v *serviceContract) IsProviderApproved(ctx context.Context, providerId *big.Int) (bool, error) {
	approved, err := v.viewContract.IsProviderApproved(&bind.CallOpts{Context: ctx}, providerId)
	if err != nil {
		return false, fmt.Errorf("failed to check if provider %s is approved: %w", providerId.String(), err)
	}
	return approved, nil
}

func (v *serviceContract) NextPDPChallengeWindowStart(ctx context.Context, proofSetID *big.Int) (*big.Int, error) {
	return v.viewContract.NextPDPChallengeWindowStart(&bind.CallOpts{Context: ctx}, proofSetID)
}

// GetApprovedProviders returns list of approved provider IDs with pagination
// offset: starting index (0-based)
// limit: maximum number of results to return
func (v *serviceContract) GetApprovedProviders(ctx context.Context, offset, limit *big.Int) ([]*big.Int, error) {
	providerIds, err := v.viewContract.GetApprovedProviders(&bind.CallOpts{Context: ctx}, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get approved providers: %w", err)
	}
	return providerIds, nil
}

// GetAllApprovedProviders returns all approved provider IDs (convenience function)
func (v *serviceContract) GetAllApprovedProviders(ctx context.Context) ([]*big.Int, error) {
	// Get the total count first
	count, err := v.viewContract.GetApprovedProvidersLength(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("failed to get approved providers count: %w", err)
	}

	// Get all providers in one call
	// TODO if this list becomes long add pagination
	return v.GetApprovedProviders(ctx, big.NewInt(0), count)
}

// GetMaxProvingPeriod returns the maximum proving period in epochs
func (v *serviceContract) GetMaxProvingPeriod(ctx context.Context) (*big.Int, error) {
	maxPeriod, err := v.viewContract.GetMaxProvingPeriod(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("failed to get max proving period: %w", err)
	}
	// Convert uint64 to *big.Int
	return new(big.Int).SetUint64(maxPeriod), nil
}

// GetChallengeWindow returns the challenge window size in epochs
func (v *serviceContract) GetChallengeWindow(ctx context.Context) (*big.Int, error) {
	window, err := v.viewContract.ChallengeWindow(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge window: %w", err)
	}
	return window, nil
}

func (v *serviceContract) Address() common.Address {
	return v.address
}
