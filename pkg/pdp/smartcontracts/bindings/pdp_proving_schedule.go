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

// PDPProvingScheduleMetaData contains all meta data concerning the PDPProvingSchedule contract.
var PDPProvingScheduleMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"getPDPConfig\",\"inputs\":[],\"outputs\":[{\"name\":\"maxProvingPeriod\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"challengeWindow\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"challengesPerProof\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"initChallengeWindowStart\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"nextPDPChallengeWindowStart\",\"inputs\":[{\"name\":\"setId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"}]",
}

// PDPProvingScheduleABI is the input ABI used to generate the binding from.
// Deprecated: Use PDPProvingScheduleMetaData.ABI instead.
var PDPProvingScheduleABI = PDPProvingScheduleMetaData.ABI

// PDPProvingSchedule is an auto generated Go binding around an Ethereum contract.
type PDPProvingSchedule struct {
	PDPProvingScheduleCaller     // Read-only binding to the contract
	PDPProvingScheduleTransactor // Write-only binding to the contract
	PDPProvingScheduleFilterer   // Log filterer for contract events
}

// PDPProvingScheduleCaller is an auto generated read-only Go binding around an Ethereum contract.
type PDPProvingScheduleCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PDPProvingScheduleTransactor is an auto generated write-only Go binding around an Ethereum contract.
type PDPProvingScheduleTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PDPProvingScheduleFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type PDPProvingScheduleFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PDPProvingScheduleSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type PDPProvingScheduleSession struct {
	Contract     *PDPProvingSchedule // Generic contract binding to set the session for
	CallOpts     bind.CallOpts       // Call options to use throughout this session
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// PDPProvingScheduleCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type PDPProvingScheduleCallerSession struct {
	Contract *PDPProvingScheduleCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts             // Call options to use throughout this session
}

// PDPProvingScheduleTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type PDPProvingScheduleTransactorSession struct {
	Contract     *PDPProvingScheduleTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts             // Transaction auth options to use throughout this session
}

// PDPProvingScheduleRaw is an auto generated low-level Go binding around an Ethereum contract.
type PDPProvingScheduleRaw struct {
	Contract *PDPProvingSchedule // Generic contract binding to access the raw methods on
}

// PDPProvingScheduleCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type PDPProvingScheduleCallerRaw struct {
	Contract *PDPProvingScheduleCaller // Generic read-only contract binding to access the raw methods on
}

// PDPProvingScheduleTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type PDPProvingScheduleTransactorRaw struct {
	Contract *PDPProvingScheduleTransactor // Generic write-only contract binding to access the raw methods on
}

// NewPDPProvingSchedule creates a new instance of PDPProvingSchedule, bound to a specific deployed contract.
func NewPDPProvingSchedule(address common.Address, backend bind.ContractBackend) (*PDPProvingSchedule, error) {
	contract, err := bindPDPProvingSchedule(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &PDPProvingSchedule{PDPProvingScheduleCaller: PDPProvingScheduleCaller{contract: contract}, PDPProvingScheduleTransactor: PDPProvingScheduleTransactor{contract: contract}, PDPProvingScheduleFilterer: PDPProvingScheduleFilterer{contract: contract}}, nil
}

// NewPDPProvingScheduleCaller creates a new read-only instance of PDPProvingSchedule, bound to a specific deployed contract.
func NewPDPProvingScheduleCaller(address common.Address, caller bind.ContractCaller) (*PDPProvingScheduleCaller, error) {
	contract, err := bindPDPProvingSchedule(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &PDPProvingScheduleCaller{contract: contract}, nil
}

// NewPDPProvingScheduleTransactor creates a new write-only instance of PDPProvingSchedule, bound to a specific deployed contract.
func NewPDPProvingScheduleTransactor(address common.Address, transactor bind.ContractTransactor) (*PDPProvingScheduleTransactor, error) {
	contract, err := bindPDPProvingSchedule(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &PDPProvingScheduleTransactor{contract: contract}, nil
}

// NewPDPProvingScheduleFilterer creates a new log filterer instance of PDPProvingSchedule, bound to a specific deployed contract.
func NewPDPProvingScheduleFilterer(address common.Address, filterer bind.ContractFilterer) (*PDPProvingScheduleFilterer, error) {
	contract, err := bindPDPProvingSchedule(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &PDPProvingScheduleFilterer{contract: contract}, nil
}

// bindPDPProvingSchedule binds a generic wrapper to an already deployed contract.
func bindPDPProvingSchedule(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := PDPProvingScheduleMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_PDPProvingSchedule *PDPProvingScheduleRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _PDPProvingSchedule.Contract.PDPProvingScheduleCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_PDPProvingSchedule *PDPProvingScheduleRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _PDPProvingSchedule.Contract.PDPProvingScheduleTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_PDPProvingSchedule *PDPProvingScheduleRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _PDPProvingSchedule.Contract.PDPProvingScheduleTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_PDPProvingSchedule *PDPProvingScheduleCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _PDPProvingSchedule.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_PDPProvingSchedule *PDPProvingScheduleTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _PDPProvingSchedule.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_PDPProvingSchedule *PDPProvingScheduleTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _PDPProvingSchedule.Contract.contract.Transact(opts, method, params...)
}

// GetPDPConfig is a free data retrieval call binding the contract method 0xea0f9354.
//
// Solidity: function getPDPConfig() view returns(uint64 maxProvingPeriod, uint256 challengeWindow, uint256 challengesPerProof, uint256 initChallengeWindowStart)
func (_PDPProvingSchedule *PDPProvingScheduleCaller) GetPDPConfig(opts *bind.CallOpts) (struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}, error) {
	var out []interface{}
	err := _PDPProvingSchedule.contract.Call(opts, &out, "getPDPConfig")

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
// Solidity: function getPDPConfig() view returns(uint64 maxProvingPeriod, uint256 challengeWindow, uint256 challengesPerProof, uint256 initChallengeWindowStart)
func (_PDPProvingSchedule *PDPProvingScheduleSession) GetPDPConfig() (struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}, error) {
	return _PDPProvingSchedule.Contract.GetPDPConfig(&_PDPProvingSchedule.CallOpts)
}

// GetPDPConfig is a free data retrieval call binding the contract method 0xea0f9354.
//
// Solidity: function getPDPConfig() view returns(uint64 maxProvingPeriod, uint256 challengeWindow, uint256 challengesPerProof, uint256 initChallengeWindowStart)
func (_PDPProvingSchedule *PDPProvingScheduleCallerSession) GetPDPConfig() (struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}, error) {
	return _PDPProvingSchedule.Contract.GetPDPConfig(&_PDPProvingSchedule.CallOpts)
}

// NextPDPChallengeWindowStart is a free data retrieval call binding the contract method 0x11d41294.
//
// Solidity: function nextPDPChallengeWindowStart(uint256 setId) view returns(uint256)
func (_PDPProvingSchedule *PDPProvingScheduleCaller) NextPDPChallengeWindowStart(opts *bind.CallOpts, setId *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _PDPProvingSchedule.contract.Call(opts, &out, "nextPDPChallengeWindowStart", setId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NextPDPChallengeWindowStart is a free data retrieval call binding the contract method 0x11d41294.
//
// Solidity: function nextPDPChallengeWindowStart(uint256 setId) view returns(uint256)
func (_PDPProvingSchedule *PDPProvingScheduleSession) NextPDPChallengeWindowStart(setId *big.Int) (*big.Int, error) {
	return _PDPProvingSchedule.Contract.NextPDPChallengeWindowStart(&_PDPProvingSchedule.CallOpts, setId)
}

// NextPDPChallengeWindowStart is a free data retrieval call binding the contract method 0x11d41294.
//
// Solidity: function nextPDPChallengeWindowStart(uint256 setId) view returns(uint256)
func (_PDPProvingSchedule *PDPProvingScheduleCallerSession) NextPDPChallengeWindowStart(setId *big.Int) (*big.Int, error) {
	return _PDPProvingSchedule.Contract.NextPDPChallengeWindowStart(&_PDPProvingSchedule.CallOpts, setId)
}
