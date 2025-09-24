// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// CidsCid is an auto generated low-level Go binding around an user-defined struct.

// SimplePDPServiceMetaData contains all meta data concerning the SimplePDPService contract.
var SimplePDPServiceMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"NO_CHALLENGE_SCHEDULED\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"NO_PROVING_DEADLINE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"UPGRADE_INTERFACE_VERSION\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"challengeWindow\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"dataSetCreated\",\"inputs\":[{\"name\":\"dataSetId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"creator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"dataSetDeleted\",\"inputs\":[{\"name\":\"dataSetId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"deletedLeafCount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"getChallengesPerProof\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"getMaxProvingPeriod\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"getPDPConfig\",\"inputs\":[],\"outputs\":[{\"name\":\"maxProvingPeriod\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"challengeWindow_\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"challengesPerProof\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"initChallengeWindowStart_\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"initChallengeWindowStart\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"initialize\",\"inputs\":[{\"name\":\"_pdpVerifierAddress\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"nextChallengeWindowStart\",\"inputs\":[{\"name\":\"setId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"nextPDPChallengeWindowStart\",\"inputs\":[{\"name\":\"setId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"nextProvingPeriod\",\"inputs\":[{\"name\":\"dataSetId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"challengeEpoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"pdpVerifierAddress\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"piecesAdded\",\"inputs\":[{\"name\":\"dataSetId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"firstAdded\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"pieceData\",\"type\":\"tuple[]\",\"internalType\":\"structCids.Cid[]\",\"components\":[{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}]},{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"piecesScheduledRemove\",\"inputs\":[{\"name\":\"dataSetId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"pieceIds\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"},{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"possessionProven\",\"inputs\":[{\"name\":\"dataSetId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"challengeCount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"provenThisPeriod\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"provingDeadlines\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"proxiableUUID\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"renounceOwnership\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"storageProviderChanged\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"thisChallengeWindowStart\",\"inputs\":[{\"name\":\"setId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"transferOwnership\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"upgradeToAndCall\",\"inputs\":[{\"name\":\"newImplementation\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"event\",\"name\":\"FaultRecord\",\"inputs\":[{\"name\":\"dataSetId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"periodsFaulted\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"deadline\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Initialized\",\"inputs\":[{\"name\":\"version\",\"type\":\"uint64\",\"indexed\":false,\"internalType\":\"uint64\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"OwnershipTransferred\",\"inputs\":[{\"name\":\"previousOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"newOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Upgraded\",\"inputs\":[{\"name\":\"implementation\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AddressEmptyCode\",\"inputs\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC1967InvalidImplementation\",\"inputs\":[{\"name\":\"implementation\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC1967NonPayable\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"FailedCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidInitialization\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotInitializing\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"OwnableInvalidOwner\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"OwnableUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"UUPSUnauthorizedCallContext\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"UUPSUnsupportedProxiableUUID\",\"inputs\":[{\"name\":\"slot\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]}]",
}

// SimplePDPServiceABI is the input ABI used to generate the binding from.
// Deprecated: Use SimplePDPServiceMetaData.ABI instead.
var SimplePDPServiceABI = SimplePDPServiceMetaData.ABI

// SimplePDPService is an auto generated Go binding around an Ethereum contract.
type SimplePDPService struct {
	SimplePDPServiceCaller     // Read-only binding to the contract
	SimplePDPServiceTransactor // Write-only binding to the contract
	SimplePDPServiceFilterer   // Log filterer for contract events
}

// SimplePDPServiceCaller is an auto generated read-only Go binding around an Ethereum contract.
type SimplePDPServiceCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SimplePDPServiceTransactor is an auto generated write-only Go binding around an Ethereum contract.
type SimplePDPServiceTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SimplePDPServiceFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SimplePDPServiceFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SimplePDPServiceSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SimplePDPServiceSession struct {
	Contract     *SimplePDPService // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// SimplePDPServiceCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SimplePDPServiceCallerSession struct {
	Contract *SimplePDPServiceCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// SimplePDPServiceTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SimplePDPServiceTransactorSession struct {
	Contract     *SimplePDPServiceTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// SimplePDPServiceRaw is an auto generated low-level Go binding around an Ethereum contract.
type SimplePDPServiceRaw struct {
	Contract *SimplePDPService // Generic contract binding to access the raw methods on
}

// SimplePDPServiceCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SimplePDPServiceCallerRaw struct {
	Contract *SimplePDPServiceCaller // Generic read-only contract binding to access the raw methods on
}

// SimplePDPServiceTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SimplePDPServiceTransactorRaw struct {
	Contract *SimplePDPServiceTransactor // Generic write-only contract binding to access the raw methods on
}

// NewSimplePDPService creates a new instance of SimplePDPService, bound to a specific deployed contract.
func NewSimplePDPService(address common.Address, backend bind.ContractBackend) (*SimplePDPService, error) {
	contract, err := bindSimplePDPService(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &SimplePDPService{SimplePDPServiceCaller: SimplePDPServiceCaller{contract: contract}, SimplePDPServiceTransactor: SimplePDPServiceTransactor{contract: contract}, SimplePDPServiceFilterer: SimplePDPServiceFilterer{contract: contract}}, nil
}

// NewSimplePDPServiceCaller creates a new read-only instance of SimplePDPService, bound to a specific deployed contract.
func NewSimplePDPServiceCaller(address common.Address, caller bind.ContractCaller) (*SimplePDPServiceCaller, error) {
	contract, err := bindSimplePDPService(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SimplePDPServiceCaller{contract: contract}, nil
}

// NewSimplePDPServiceTransactor creates a new write-only instance of SimplePDPService, bound to a specific deployed contract.
func NewSimplePDPServiceTransactor(address common.Address, transactor bind.ContractTransactor) (*SimplePDPServiceTransactor, error) {
	contract, err := bindSimplePDPService(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SimplePDPServiceTransactor{contract: contract}, nil
}

// NewSimplePDPServiceFilterer creates a new log filterer instance of SimplePDPService, bound to a specific deployed contract.
func NewSimplePDPServiceFilterer(address common.Address, filterer bind.ContractFilterer) (*SimplePDPServiceFilterer, error) {
	contract, err := bindSimplePDPService(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SimplePDPServiceFilterer{contract: contract}, nil
}

// bindSimplePDPService binds a generic wrapper to an already deployed contract.
func bindSimplePDPService(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := SimplePDPServiceMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SimplePDPService *SimplePDPServiceRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SimplePDPService.Contract.SimplePDPServiceCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SimplePDPService *SimplePDPServiceRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SimplePDPService.Contract.SimplePDPServiceTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SimplePDPService *SimplePDPServiceRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SimplePDPService.Contract.SimplePDPServiceTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SimplePDPService *SimplePDPServiceCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SimplePDPService.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SimplePDPService *SimplePDPServiceTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SimplePDPService.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SimplePDPService *SimplePDPServiceTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SimplePDPService.Contract.contract.Transact(opts, method, params...)
}

// NOCHALLENGESCHEDULED is a free data retrieval call binding the contract method 0x462dd449.
//
// Solidity: function NO_CHALLENGE_SCHEDULED() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCaller) NOCHALLENGESCHEDULED(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "NO_CHALLENGE_SCHEDULED")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NOCHALLENGESCHEDULED is a free data retrieval call binding the contract method 0x462dd449.
//
// Solidity: function NO_CHALLENGE_SCHEDULED() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceSession) NOCHALLENGESCHEDULED() (*big.Int, error) {
	return _SimplePDPService.Contract.NOCHALLENGESCHEDULED(&_SimplePDPService.CallOpts)
}

// NOCHALLENGESCHEDULED is a free data retrieval call binding the contract method 0x462dd449.
//
// Solidity: function NO_CHALLENGE_SCHEDULED() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCallerSession) NOCHALLENGESCHEDULED() (*big.Int, error) {
	return _SimplePDPService.Contract.NOCHALLENGESCHEDULED(&_SimplePDPService.CallOpts)
}

// NOPROVINGDEADLINE is a free data retrieval call binding the contract method 0x4ba9ac22.
//
// Solidity: function NO_PROVING_DEADLINE() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCaller) NOPROVINGDEADLINE(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "NO_PROVING_DEADLINE")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NOPROVINGDEADLINE is a free data retrieval call binding the contract method 0x4ba9ac22.
//
// Solidity: function NO_PROVING_DEADLINE() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceSession) NOPROVINGDEADLINE() (*big.Int, error) {
	return _SimplePDPService.Contract.NOPROVINGDEADLINE(&_SimplePDPService.CallOpts)
}

// NOPROVINGDEADLINE is a free data retrieval call binding the contract method 0x4ba9ac22.
//
// Solidity: function NO_PROVING_DEADLINE() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCallerSession) NOPROVINGDEADLINE() (*big.Int, error) {
	return _SimplePDPService.Contract.NOPROVINGDEADLINE(&_SimplePDPService.CallOpts)
}

// UPGRADEINTERFACEVERSION is a free data retrieval call binding the contract method 0xad3cb1cc.
//
// Solidity: function UPGRADE_INTERFACE_VERSION() view returns(string)
func (_SimplePDPService *SimplePDPServiceCaller) UPGRADEINTERFACEVERSION(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "UPGRADE_INTERFACE_VERSION")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// UPGRADEINTERFACEVERSION is a free data retrieval call binding the contract method 0xad3cb1cc.
//
// Solidity: function UPGRADE_INTERFACE_VERSION() view returns(string)
func (_SimplePDPService *SimplePDPServiceSession) UPGRADEINTERFACEVERSION() (string, error) {
	return _SimplePDPService.Contract.UPGRADEINTERFACEVERSION(&_SimplePDPService.CallOpts)
}

// UPGRADEINTERFACEVERSION is a free data retrieval call binding the contract method 0xad3cb1cc.
//
// Solidity: function UPGRADE_INTERFACE_VERSION() view returns(string)
func (_SimplePDPService *SimplePDPServiceCallerSession) UPGRADEINTERFACEVERSION() (string, error) {
	return _SimplePDPService.Contract.UPGRADEINTERFACEVERSION(&_SimplePDPService.CallOpts)
}

// ChallengeWindow is a free data retrieval call binding the contract method 0x861a1412.
//
// Solidity: function challengeWindow() pure returns(uint256)
func (_SimplePDPService *SimplePDPServiceCaller) ChallengeWindow(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "challengeWindow")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ChallengeWindow is a free data retrieval call binding the contract method 0x861a1412.
//
// Solidity: function challengeWindow() pure returns(uint256)
func (_SimplePDPService *SimplePDPServiceSession) ChallengeWindow() (*big.Int, error) {
	return _SimplePDPService.Contract.ChallengeWindow(&_SimplePDPService.CallOpts)
}

// ChallengeWindow is a free data retrieval call binding the contract method 0x861a1412.
//
// Solidity: function challengeWindow() pure returns(uint256)
func (_SimplePDPService *SimplePDPServiceCallerSession) ChallengeWindow() (*big.Int, error) {
	return _SimplePDPService.Contract.ChallengeWindow(&_SimplePDPService.CallOpts)
}

// GetChallengesPerProof is a free data retrieval call binding the contract method 0x47d3dfe7.
//
// Solidity: function getChallengesPerProof() pure returns(uint64)
func (_SimplePDPService *SimplePDPServiceCaller) GetChallengesPerProof(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "getChallengesPerProof")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// GetChallengesPerProof is a free data retrieval call binding the contract method 0x47d3dfe7.
//
// Solidity: function getChallengesPerProof() pure returns(uint64)
func (_SimplePDPService *SimplePDPServiceSession) GetChallengesPerProof() (uint64, error) {
	return _SimplePDPService.Contract.GetChallengesPerProof(&_SimplePDPService.CallOpts)
}

// GetChallengesPerProof is a free data retrieval call binding the contract method 0x47d3dfe7.
//
// Solidity: function getChallengesPerProof() pure returns(uint64)
func (_SimplePDPService *SimplePDPServiceCallerSession) GetChallengesPerProof() (uint64, error) {
	return _SimplePDPService.Contract.GetChallengesPerProof(&_SimplePDPService.CallOpts)
}

// GetMaxProvingPeriod is a free data retrieval call binding the contract method 0xf2f12333.
//
// Solidity: function getMaxProvingPeriod() pure returns(uint64)
func (_SimplePDPService *SimplePDPServiceCaller) GetMaxProvingPeriod(opts *bind.CallOpts) (uint64, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "getMaxProvingPeriod")

	if err != nil {
		return *new(uint64), err
	}

	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)

	return out0, err

}

// GetMaxProvingPeriod is a free data retrieval call binding the contract method 0xf2f12333.
//
// Solidity: function getMaxProvingPeriod() pure returns(uint64)
func (_SimplePDPService *SimplePDPServiceSession) GetMaxProvingPeriod() (uint64, error) {
	return _SimplePDPService.Contract.GetMaxProvingPeriod(&_SimplePDPService.CallOpts)
}

// GetMaxProvingPeriod is a free data retrieval call binding the contract method 0xf2f12333.
//
// Solidity: function getMaxProvingPeriod() pure returns(uint64)
func (_SimplePDPService *SimplePDPServiceCallerSession) GetMaxProvingPeriod() (uint64, error) {
	return _SimplePDPService.Contract.GetMaxProvingPeriod(&_SimplePDPService.CallOpts)
}

// GetPDPConfig is a free data retrieval call binding the contract method 0xea0f9354.
//
// Solidity: function getPDPConfig() view returns(uint64 maxProvingPeriod, uint256 challengeWindow_, uint256 challengesPerProof, uint256 initChallengeWindowStart_)
func (_SimplePDPService *SimplePDPServiceCaller) GetPDPConfig(opts *bind.CallOpts) (struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "getPDPConfig")

	outstruct := new(struct {
		MaxProvingPeriod         uint64
		ChallengeWindow          *big.Int
		ChallengesPerProof       *big.Int
		InitChallengeWindowStart *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.MaxProvingPeriod = *abi.ConvertType(out[0], new(uint64)).(*uint64)
	outstruct.ChallengeWindow = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.ChallengesPerProof = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.InitChallengeWindowStart = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetPDPConfig is a free data retrieval call binding the contract method 0xea0f9354.
//
// Solidity: function getPDPConfig() view returns(uint64 maxProvingPeriod, uint256 challengeWindow_, uint256 challengesPerProof, uint256 initChallengeWindowStart_)
func (_SimplePDPService *SimplePDPServiceSession) GetPDPConfig() (struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}, error) {
	return _SimplePDPService.Contract.GetPDPConfig(&_SimplePDPService.CallOpts)
}

// GetPDPConfig is a free data retrieval call binding the contract method 0xea0f9354.
//
// Solidity: function getPDPConfig() view returns(uint64 maxProvingPeriod, uint256 challengeWindow_, uint256 challengesPerProof, uint256 initChallengeWindowStart_)
func (_SimplePDPService *SimplePDPServiceCallerSession) GetPDPConfig() (struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}, error) {
	return _SimplePDPService.Contract.GetPDPConfig(&_SimplePDPService.CallOpts)
}

// InitChallengeWindowStart is a free data retrieval call binding the contract method 0x21918cea.
//
// Solidity: function initChallengeWindowStart() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCaller) InitChallengeWindowStart(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "initChallengeWindowStart")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// InitChallengeWindowStart is a free data retrieval call binding the contract method 0x21918cea.
//
// Solidity: function initChallengeWindowStart() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceSession) InitChallengeWindowStart() (*big.Int, error) {
	return _SimplePDPService.Contract.InitChallengeWindowStart(&_SimplePDPService.CallOpts)
}

// InitChallengeWindowStart is a free data retrieval call binding the contract method 0x21918cea.
//
// Solidity: function initChallengeWindowStart() view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCallerSession) InitChallengeWindowStart() (*big.Int, error) {
	return _SimplePDPService.Contract.InitChallengeWindowStart(&_SimplePDPService.CallOpts)
}

// NextChallengeWindowStart is a free data retrieval call binding the contract method 0x8bf96d28.
//
// Solidity: function nextChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCaller) NextChallengeWindowStart(opts *bind.CallOpts, setId *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "nextChallengeWindowStart", setId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NextChallengeWindowStart is a free data retrieval call binding the contract method 0x8bf96d28.
//
// Solidity: function nextChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceSession) NextChallengeWindowStart(setId *big.Int) (*big.Int, error) {
	return _SimplePDPService.Contract.NextChallengeWindowStart(&_SimplePDPService.CallOpts, setId)
}

// NextChallengeWindowStart is a free data retrieval call binding the contract method 0x8bf96d28.
//
// Solidity: function nextChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCallerSession) NextChallengeWindowStart(setId *big.Int) (*big.Int, error) {
	return _SimplePDPService.Contract.NextChallengeWindowStart(&_SimplePDPService.CallOpts, setId)
}

// NextPDPChallengeWindowStart is a free data retrieval call binding the contract method 0x11d41294.
//
// Solidity: function nextPDPChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCaller) NextPDPChallengeWindowStart(opts *bind.CallOpts, setId *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "nextPDPChallengeWindowStart", setId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NextPDPChallengeWindowStart is a free data retrieval call binding the contract method 0x11d41294.
//
// Solidity: function nextPDPChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceSession) NextPDPChallengeWindowStart(setId *big.Int) (*big.Int, error) {
	return _SimplePDPService.Contract.NextPDPChallengeWindowStart(&_SimplePDPService.CallOpts, setId)
}

// NextPDPChallengeWindowStart is a free data retrieval call binding the contract method 0x11d41294.
//
// Solidity: function nextPDPChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCallerSession) NextPDPChallengeWindowStart(setId *big.Int) (*big.Int, error) {
	return _SimplePDPService.Contract.NextPDPChallengeWindowStart(&_SimplePDPService.CallOpts, setId)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SimplePDPService *SimplePDPServiceCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SimplePDPService *SimplePDPServiceSession) Owner() (common.Address, error) {
	return _SimplePDPService.Contract.Owner(&_SimplePDPService.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SimplePDPService *SimplePDPServiceCallerSession) Owner() (common.Address, error) {
	return _SimplePDPService.Contract.Owner(&_SimplePDPService.CallOpts)
}

// PdpVerifierAddress is a free data retrieval call binding the contract method 0xde4b6b71.
//
// Solidity: function pdpVerifierAddress() view returns(address)
func (_SimplePDPService *SimplePDPServiceCaller) PdpVerifierAddress(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "pdpVerifierAddress")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// PdpVerifierAddress is a free data retrieval call binding the contract method 0xde4b6b71.
//
// Solidity: function pdpVerifierAddress() view returns(address)
func (_SimplePDPService *SimplePDPServiceSession) PdpVerifierAddress() (common.Address, error) {
	return _SimplePDPService.Contract.PdpVerifierAddress(&_SimplePDPService.CallOpts)
}

// PdpVerifierAddress is a free data retrieval call binding the contract method 0xde4b6b71.
//
// Solidity: function pdpVerifierAddress() view returns(address)
func (_SimplePDPService *SimplePDPServiceCallerSession) PdpVerifierAddress() (common.Address, error) {
	return _SimplePDPService.Contract.PdpVerifierAddress(&_SimplePDPService.CallOpts)
}

// ProvenThisPeriod is a free data retrieval call binding the contract method 0x7598a1cd.
//
// Solidity: function provenThisPeriod(uint256 ) view returns(bool)
func (_SimplePDPService *SimplePDPServiceCaller) ProvenThisPeriod(opts *bind.CallOpts, arg0 *big.Int) (bool, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "provenThisPeriod", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ProvenThisPeriod is a free data retrieval call binding the contract method 0x7598a1cd.
//
// Solidity: function provenThisPeriod(uint256 ) view returns(bool)
func (_SimplePDPService *SimplePDPServiceSession) ProvenThisPeriod(arg0 *big.Int) (bool, error) {
	return _SimplePDPService.Contract.ProvenThisPeriod(&_SimplePDPService.CallOpts, arg0)
}

// ProvenThisPeriod is a free data retrieval call binding the contract method 0x7598a1cd.
//
// Solidity: function provenThisPeriod(uint256 ) view returns(bool)
func (_SimplePDPService *SimplePDPServiceCallerSession) ProvenThisPeriod(arg0 *big.Int) (bool, error) {
	return _SimplePDPService.Contract.ProvenThisPeriod(&_SimplePDPService.CallOpts, arg0)
}

// ProvingDeadlines is a free data retrieval call binding the contract method 0x11bc4865.
//
// Solidity: function provingDeadlines(uint256 ) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCaller) ProvingDeadlines(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "provingDeadlines", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ProvingDeadlines is a free data retrieval call binding the contract method 0x11bc4865.
//
// Solidity: function provingDeadlines(uint256 ) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceSession) ProvingDeadlines(arg0 *big.Int) (*big.Int, error) {
	return _SimplePDPService.Contract.ProvingDeadlines(&_SimplePDPService.CallOpts, arg0)
}

// ProvingDeadlines is a free data retrieval call binding the contract method 0x11bc4865.
//
// Solidity: function provingDeadlines(uint256 ) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCallerSession) ProvingDeadlines(arg0 *big.Int) (*big.Int, error) {
	return _SimplePDPService.Contract.ProvingDeadlines(&_SimplePDPService.CallOpts, arg0)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_SimplePDPService *SimplePDPServiceCaller) ProxiableUUID(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "proxiableUUID")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_SimplePDPService *SimplePDPServiceSession) ProxiableUUID() ([32]byte, error) {
	return _SimplePDPService.Contract.ProxiableUUID(&_SimplePDPService.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_SimplePDPService *SimplePDPServiceCallerSession) ProxiableUUID() ([32]byte, error) {
	return _SimplePDPService.Contract.ProxiableUUID(&_SimplePDPService.CallOpts)
}

// ThisChallengeWindowStart is a free data retrieval call binding the contract method 0x1506d198.
//
// Solidity: function thisChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCaller) ThisChallengeWindowStart(opts *bind.CallOpts, setId *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SimplePDPService.contract.Call(opts, &out, "thisChallengeWindowStart", setId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ThisChallengeWindowStart is a free data retrieval call binding the contract method 0x1506d198.
//
// Solidity: function thisChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceSession) ThisChallengeWindowStart(setId *big.Int) (*big.Int, error) {
	return _SimplePDPService.Contract.ThisChallengeWindowStart(&_SimplePDPService.CallOpts, setId)
}

// ThisChallengeWindowStart is a free data retrieval call binding the contract method 0x1506d198.
//
// Solidity: function thisChallengeWindowStart(uint256 setId) view returns(uint256)
func (_SimplePDPService *SimplePDPServiceCallerSession) ThisChallengeWindowStart(setId *big.Int) (*big.Int, error) {
	return _SimplePDPService.Contract.ThisChallengeWindowStart(&_SimplePDPService.CallOpts, setId)
}

// DataSetCreated is a paid mutator transaction binding the contract method 0x101c1eab.
//
// Solidity: function dataSetCreated(uint256 dataSetId, address creator, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) DataSetCreated(opts *bind.TransactOpts, dataSetId *big.Int, creator common.Address, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "dataSetCreated", dataSetId, creator, arg2)
}

// DataSetCreated is a paid mutator transaction binding the contract method 0x101c1eab.
//
// Solidity: function dataSetCreated(uint256 dataSetId, address creator, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceSession) DataSetCreated(dataSetId *big.Int, creator common.Address, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.DataSetCreated(&_SimplePDPService.TransactOpts, dataSetId, creator, arg2)
}

// DataSetCreated is a paid mutator transaction binding the contract method 0x101c1eab.
//
// Solidity: function dataSetCreated(uint256 dataSetId, address creator, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) DataSetCreated(dataSetId *big.Int, creator common.Address, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.DataSetCreated(&_SimplePDPService.TransactOpts, dataSetId, creator, arg2)
}

// DataSetDeleted is a paid mutator transaction binding the contract method 0x2abd465c.
//
// Solidity: function dataSetDeleted(uint256 dataSetId, uint256 deletedLeafCount, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) DataSetDeleted(opts *bind.TransactOpts, dataSetId *big.Int, deletedLeafCount *big.Int, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "dataSetDeleted", dataSetId, deletedLeafCount, arg2)
}

// DataSetDeleted is a paid mutator transaction binding the contract method 0x2abd465c.
//
// Solidity: function dataSetDeleted(uint256 dataSetId, uint256 deletedLeafCount, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceSession) DataSetDeleted(dataSetId *big.Int, deletedLeafCount *big.Int, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.DataSetDeleted(&_SimplePDPService.TransactOpts, dataSetId, deletedLeafCount, arg2)
}

// DataSetDeleted is a paid mutator transaction binding the contract method 0x2abd465c.
//
// Solidity: function dataSetDeleted(uint256 dataSetId, uint256 deletedLeafCount, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) DataSetDeleted(dataSetId *big.Int, deletedLeafCount *big.Int, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.DataSetDeleted(&_SimplePDPService.TransactOpts, dataSetId, deletedLeafCount, arg2)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _pdpVerifierAddress) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) Initialize(opts *bind.TransactOpts, _pdpVerifierAddress common.Address) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "initialize", _pdpVerifierAddress)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _pdpVerifierAddress) returns()
func (_SimplePDPService *SimplePDPServiceSession) Initialize(_pdpVerifierAddress common.Address) (*types.Transaction, error) {
	return _SimplePDPService.Contract.Initialize(&_SimplePDPService.TransactOpts, _pdpVerifierAddress)
}

// Initialize is a paid mutator transaction binding the contract method 0xc4d66de8.
//
// Solidity: function initialize(address _pdpVerifierAddress) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) Initialize(_pdpVerifierAddress common.Address) (*types.Transaction, error) {
	return _SimplePDPService.Contract.Initialize(&_SimplePDPService.TransactOpts, _pdpVerifierAddress)
}

// NextProvingPeriod is a paid mutator transaction binding the contract method 0xaa27ebcc.
//
// Solidity: function nextProvingPeriod(uint256 dataSetId, uint256 challengeEpoch, uint256 , bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) NextProvingPeriod(opts *bind.TransactOpts, dataSetId *big.Int, challengeEpoch *big.Int, arg2 *big.Int, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "nextProvingPeriod", dataSetId, challengeEpoch, arg2, arg3)
}

// NextProvingPeriod is a paid mutator transaction binding the contract method 0xaa27ebcc.
//
// Solidity: function nextProvingPeriod(uint256 dataSetId, uint256 challengeEpoch, uint256 , bytes ) returns()
func (_SimplePDPService *SimplePDPServiceSession) NextProvingPeriod(dataSetId *big.Int, challengeEpoch *big.Int, arg2 *big.Int, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.NextProvingPeriod(&_SimplePDPService.TransactOpts, dataSetId, challengeEpoch, arg2, arg3)
}

// NextProvingPeriod is a paid mutator transaction binding the contract method 0xaa27ebcc.
//
// Solidity: function nextProvingPeriod(uint256 dataSetId, uint256 challengeEpoch, uint256 , bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) NextProvingPeriod(dataSetId *big.Int, challengeEpoch *big.Int, arg2 *big.Int, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.NextProvingPeriod(&_SimplePDPService.TransactOpts, dataSetId, challengeEpoch, arg2, arg3)
}

// PiecesAdded is a paid mutator transaction binding the contract method 0xf6814d79.
//
// Solidity: function piecesAdded(uint256 dataSetId, uint256 firstAdded, (bytes)[] pieceData, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) PiecesAdded(opts *bind.TransactOpts, dataSetId *big.Int, firstAdded *big.Int, pieceData []CidsCid, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "piecesAdded", dataSetId, firstAdded, pieceData, arg3)
}

// PiecesAdded is a paid mutator transaction binding the contract method 0xf6814d79.
//
// Solidity: function piecesAdded(uint256 dataSetId, uint256 firstAdded, (bytes)[] pieceData, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceSession) PiecesAdded(dataSetId *big.Int, firstAdded *big.Int, pieceData []CidsCid, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.PiecesAdded(&_SimplePDPService.TransactOpts, dataSetId, firstAdded, pieceData, arg3)
}

// PiecesAdded is a paid mutator transaction binding the contract method 0xf6814d79.
//
// Solidity: function piecesAdded(uint256 dataSetId, uint256 firstAdded, (bytes)[] pieceData, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) PiecesAdded(dataSetId *big.Int, firstAdded *big.Int, pieceData []CidsCid, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.PiecesAdded(&_SimplePDPService.TransactOpts, dataSetId, firstAdded, pieceData, arg3)
}

// PiecesScheduledRemove is a paid mutator transaction binding the contract method 0xe7954aa7.
//
// Solidity: function piecesScheduledRemove(uint256 dataSetId, uint256[] pieceIds, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) PiecesScheduledRemove(opts *bind.TransactOpts, dataSetId *big.Int, pieceIds []*big.Int, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "piecesScheduledRemove", dataSetId, pieceIds, arg2)
}

// PiecesScheduledRemove is a paid mutator transaction binding the contract method 0xe7954aa7.
//
// Solidity: function piecesScheduledRemove(uint256 dataSetId, uint256[] pieceIds, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceSession) PiecesScheduledRemove(dataSetId *big.Int, pieceIds []*big.Int, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.PiecesScheduledRemove(&_SimplePDPService.TransactOpts, dataSetId, pieceIds, arg2)
}

// PiecesScheduledRemove is a paid mutator transaction binding the contract method 0xe7954aa7.
//
// Solidity: function piecesScheduledRemove(uint256 dataSetId, uint256[] pieceIds, bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) PiecesScheduledRemove(dataSetId *big.Int, pieceIds []*big.Int, arg2 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.PiecesScheduledRemove(&_SimplePDPService.TransactOpts, dataSetId, pieceIds, arg2)
}

// PossessionProven is a paid mutator transaction binding the contract method 0x356de02b.
//
// Solidity: function possessionProven(uint256 dataSetId, uint256 , uint256 , uint256 challengeCount) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) PossessionProven(opts *bind.TransactOpts, dataSetId *big.Int, arg1 *big.Int, arg2 *big.Int, challengeCount *big.Int) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "possessionProven", dataSetId, arg1, arg2, challengeCount)
}

// PossessionProven is a paid mutator transaction binding the contract method 0x356de02b.
//
// Solidity: function possessionProven(uint256 dataSetId, uint256 , uint256 , uint256 challengeCount) returns()
func (_SimplePDPService *SimplePDPServiceSession) PossessionProven(dataSetId *big.Int, arg1 *big.Int, arg2 *big.Int, challengeCount *big.Int) (*types.Transaction, error) {
	return _SimplePDPService.Contract.PossessionProven(&_SimplePDPService.TransactOpts, dataSetId, arg1, arg2, challengeCount)
}

// PossessionProven is a paid mutator transaction binding the contract method 0x356de02b.
//
// Solidity: function possessionProven(uint256 dataSetId, uint256 , uint256 , uint256 challengeCount) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) PossessionProven(dataSetId *big.Int, arg1 *big.Int, arg2 *big.Int, challengeCount *big.Int) (*types.Transaction, error) {
	return _SimplePDPService.Contract.PossessionProven(&_SimplePDPService.TransactOpts, dataSetId, arg1, arg2, challengeCount)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SimplePDPService *SimplePDPServiceTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SimplePDPService *SimplePDPServiceSession) RenounceOwnership() (*types.Transaction, error) {
	return _SimplePDPService.Contract.RenounceOwnership(&_SimplePDPService.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _SimplePDPService.Contract.RenounceOwnership(&_SimplePDPService.TransactOpts)
}

// StorageProviderChanged is a paid mutator transaction binding the contract method 0x4059b6d7.
//
// Solidity: function storageProviderChanged(uint256 , address , address , bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) StorageProviderChanged(opts *bind.TransactOpts, arg0 *big.Int, arg1 common.Address, arg2 common.Address, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "storageProviderChanged", arg0, arg1, arg2, arg3)
}

// StorageProviderChanged is a paid mutator transaction binding the contract method 0x4059b6d7.
//
// Solidity: function storageProviderChanged(uint256 , address , address , bytes ) returns()
func (_SimplePDPService *SimplePDPServiceSession) StorageProviderChanged(arg0 *big.Int, arg1 common.Address, arg2 common.Address, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.StorageProviderChanged(&_SimplePDPService.TransactOpts, arg0, arg1, arg2, arg3)
}

// StorageProviderChanged is a paid mutator transaction binding the contract method 0x4059b6d7.
//
// Solidity: function storageProviderChanged(uint256 , address , address , bytes ) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) StorageProviderChanged(arg0 *big.Int, arg1 common.Address, arg2 common.Address, arg3 []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.StorageProviderChanged(&_SimplePDPService.TransactOpts, arg0, arg1, arg2, arg3)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SimplePDPService *SimplePDPServiceTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SimplePDPService *SimplePDPServiceSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _SimplePDPService.Contract.TransferOwnership(&_SimplePDPService.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _SimplePDPService.Contract.TransferOwnership(&_SimplePDPService.TransactOpts, newOwner)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_SimplePDPService *SimplePDPServiceTransactor) UpgradeToAndCall(opts *bind.TransactOpts, newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _SimplePDPService.contract.Transact(opts, "upgradeToAndCall", newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_SimplePDPService *SimplePDPServiceSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.UpgradeToAndCall(&_SimplePDPService.TransactOpts, newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_SimplePDPService *SimplePDPServiceTransactorSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _SimplePDPService.Contract.UpgradeToAndCall(&_SimplePDPService.TransactOpts, newImplementation, data)
}

// SimplePDPServiceFaultRecordIterator is returned from FilterFaultRecord and is used to iterate over the raw logs and unpacked data for FaultRecord events raised by the SimplePDPService contract.
type SimplePDPServiceFaultRecordIterator struct {
	Event *SimplePDPServiceFaultRecord // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SimplePDPServiceFaultRecordIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SimplePDPServiceFaultRecord)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SimplePDPServiceFaultRecord)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SimplePDPServiceFaultRecordIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SimplePDPServiceFaultRecordIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SimplePDPServiceFaultRecord represents a FaultRecord event raised by the SimplePDPService contract.
type SimplePDPServiceFaultRecord struct {
	DataSetId      *big.Int
	PeriodsFaulted *big.Int
	Deadline       *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterFaultRecord is a free log retrieval operation binding the contract event 0xff5f076c63706be9f7eaafa8329db4a9ce9b9e3cd6e53470f05491e2043e1a81.
//
// Solidity: event FaultRecord(uint256 indexed dataSetId, uint256 periodsFaulted, uint256 deadline)
func (_SimplePDPService *SimplePDPServiceFilterer) FilterFaultRecord(opts *bind.FilterOpts, dataSetId []*big.Int) (*SimplePDPServiceFaultRecordIterator, error) {

	var dataSetIdRule []interface{}
	for _, dataSetIdItem := range dataSetId {
		dataSetIdRule = append(dataSetIdRule, dataSetIdItem)
	}

	logs, sub, err := _SimplePDPService.contract.FilterLogs(opts, "FaultRecord", dataSetIdRule)
	if err != nil {
		return nil, err
	}
	return &SimplePDPServiceFaultRecordIterator{contract: _SimplePDPService.contract, event: "FaultRecord", logs: logs, sub: sub}, nil
}

// WatchFaultRecord is a free log subscription operation binding the contract event 0xff5f076c63706be9f7eaafa8329db4a9ce9b9e3cd6e53470f05491e2043e1a81.
//
// Solidity: event FaultRecord(uint256 indexed dataSetId, uint256 periodsFaulted, uint256 deadline)
func (_SimplePDPService *SimplePDPServiceFilterer) WatchFaultRecord(opts *bind.WatchOpts, sink chan<- *SimplePDPServiceFaultRecord, dataSetId []*big.Int) (event.Subscription, error) {

	var dataSetIdRule []interface{}
	for _, dataSetIdItem := range dataSetId {
		dataSetIdRule = append(dataSetIdRule, dataSetIdItem)
	}

	logs, sub, err := _SimplePDPService.contract.WatchLogs(opts, "FaultRecord", dataSetIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SimplePDPServiceFaultRecord)
				if err := _SimplePDPService.contract.UnpackLog(event, "FaultRecord", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFaultRecord is a log parse operation binding the contract event 0xff5f076c63706be9f7eaafa8329db4a9ce9b9e3cd6e53470f05491e2043e1a81.
//
// Solidity: event FaultRecord(uint256 indexed dataSetId, uint256 periodsFaulted, uint256 deadline)
func (_SimplePDPService *SimplePDPServiceFilterer) ParseFaultRecord(log types.Log) (*SimplePDPServiceFaultRecord, error) {
	event := new(SimplePDPServiceFaultRecord)
	if err := _SimplePDPService.contract.UnpackLog(event, "FaultRecord", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SimplePDPServiceInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the SimplePDPService contract.
type SimplePDPServiceInitializedIterator struct {
	Event *SimplePDPServiceInitialized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SimplePDPServiceInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SimplePDPServiceInitialized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SimplePDPServiceInitialized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SimplePDPServiceInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SimplePDPServiceInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SimplePDPServiceInitialized represents a Initialized event raised by the SimplePDPService contract.
type SimplePDPServiceInitialized struct {
	Version uint64
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_SimplePDPService *SimplePDPServiceFilterer) FilterInitialized(opts *bind.FilterOpts) (*SimplePDPServiceInitializedIterator, error) {

	logs, sub, err := _SimplePDPService.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &SimplePDPServiceInitializedIterator{contract: _SimplePDPService.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_SimplePDPService *SimplePDPServiceFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *SimplePDPServiceInitialized) (event.Subscription, error) {

	logs, sub, err := _SimplePDPService.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SimplePDPServiceInitialized)
				if err := _SimplePDPService.contract.UnpackLog(event, "Initialized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInitialized is a log parse operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_SimplePDPService *SimplePDPServiceFilterer) ParseInitialized(log types.Log) (*SimplePDPServiceInitialized, error) {
	event := new(SimplePDPServiceInitialized)
	if err := _SimplePDPService.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SimplePDPServiceOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the SimplePDPService contract.
type SimplePDPServiceOwnershipTransferredIterator struct {
	Event *SimplePDPServiceOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SimplePDPServiceOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SimplePDPServiceOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SimplePDPServiceOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SimplePDPServiceOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SimplePDPServiceOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SimplePDPServiceOwnershipTransferred represents a OwnershipTransferred event raised by the SimplePDPService contract.
type SimplePDPServiceOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SimplePDPService *SimplePDPServiceFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*SimplePDPServiceOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _SimplePDPService.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &SimplePDPServiceOwnershipTransferredIterator{contract: _SimplePDPService.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SimplePDPService *SimplePDPServiceFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *SimplePDPServiceOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _SimplePDPService.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SimplePDPServiceOwnershipTransferred)
				if err := _SimplePDPService.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SimplePDPService *SimplePDPServiceFilterer) ParseOwnershipTransferred(log types.Log) (*SimplePDPServiceOwnershipTransferred, error) {
	event := new(SimplePDPServiceOwnershipTransferred)
	if err := _SimplePDPService.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SimplePDPServiceUpgradedIterator is returned from FilterUpgraded and is used to iterate over the raw logs and unpacked data for Upgraded events raised by the SimplePDPService contract.
type SimplePDPServiceUpgradedIterator struct {
	Event *SimplePDPServiceUpgraded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SimplePDPServiceUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SimplePDPServiceUpgraded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SimplePDPServiceUpgraded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SimplePDPServiceUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SimplePDPServiceUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SimplePDPServiceUpgraded represents a Upgraded event raised by the SimplePDPService contract.
type SimplePDPServiceUpgraded struct {
	Implementation common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterUpgraded is a free log retrieval operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_SimplePDPService *SimplePDPServiceFilterer) FilterUpgraded(opts *bind.FilterOpts, implementation []common.Address) (*SimplePDPServiceUpgradedIterator, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _SimplePDPService.contract.FilterLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return &SimplePDPServiceUpgradedIterator{contract: _SimplePDPService.contract, event: "Upgraded", logs: logs, sub: sub}, nil
}

// WatchUpgraded is a free log subscription operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_SimplePDPService *SimplePDPServiceFilterer) WatchUpgraded(opts *bind.WatchOpts, sink chan<- *SimplePDPServiceUpgraded, implementation []common.Address) (event.Subscription, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _SimplePDPService.contract.WatchLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SimplePDPServiceUpgraded)
				if err := _SimplePDPService.contract.UnpackLog(event, "Upgraded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUpgraded is a log parse operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_SimplePDPService *SimplePDPServiceFilterer) ParseUpgraded(log types.Log) (*SimplePDPServiceUpgraded, error) {
	event := new(SimplePDPServiceUpgraded)
	if err := _SimplePDPService.contract.UnpackLog(event, "Upgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
