package smartcontracts

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/filecoin-services/go/bindings"
)

type Service interface {
	PDPConfig(ctx context.Context) (PDPConfig, error)
	GetDataSet(ctx context.Context, dataSetId *big.Int) (*DataSetInfo, error)
	IsProviderApproved(ctx context.Context, providerId *big.Int) (bool, error)
	NextPDPChallengeWindowStart(ctx context.Context, proofSetID *big.Int) (*big.Int, error)
	RailToDataSet(ctx context.Context, railId *big.Int) (*big.Int, error)

	// not part of contract code, added for convience in testing and usage
	Address() common.Address
}

// serviceContract provides helper functions for interacting with FilecoinWarmStorageServiceStateView
type serviceContract struct {
	address      common.Address
	viewContract *bindings.FilecoinWarmStorageServiceStateView
	client       bind.ContractBackend
	dataSets     sync.Map // cache dataset lookups by ID string
}

// NewServiceView creates a new view contract helper
// It first gets the view contract address from the main service contract, then connects to it
func NewServiceView(address common.Address, client bind.ContractBackend) (Service, error) {
	// Connect to the view contract
	viewContract, err := bindings.NewFilecoinWarmStorageServiceStateView(address, client)
	if err != nil {
		return nil, fmt.Errorf("failed to bind view contract at %s: %w", address, err)
	}

	return &serviceContract{
		address:      address,
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
	key := dataSetId.String()
	if cached, ok := v.dataSets.Load(key); ok {
		return cached.(*DataSetInfo), nil
	}

	result, err := v.viewContract.GetDataSet(&bind.CallOpts{Context: ctx}, dataSetId)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset %s: %w", key, err)
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
		return nil, fmt.Errorf("dataset %s does not exist", key)
	}

	v.dataSets.Store(key, info)

	return info, nil
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

func (v *serviceContract) Address() common.Address {
	return v.address
}

func (v *serviceContract) RailToDataSet(ctx context.Context, railId *big.Int) (*big.Int, error) {
	dataSetId, err := v.viewContract.RailToDataSet(&bind.CallOpts{Context: ctx}, railId)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset for rail %s: %w", railId.String(), err)
	}
	return dataSetId, nil
}
