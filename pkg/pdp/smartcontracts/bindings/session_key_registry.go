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

// SessionKeyRegistryMetaData contains all meta data concerning the SessionKeyRegistry contract.
var SessionKeyRegistryMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"authorizationExpiry\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"signer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"permission\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"login\",\"inputs\":[{\"name\":\"signer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"expiry\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"permissions\",\"type\":\"bytes32[]\",\"internalType\":\"bytes32[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"loginAndFund\",\"inputs\":[{\"name\":\"signer\",\"type\":\"address\",\"internalType\":\"addresspayable\"},{\"name\":\"expiry\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"permissions\",\"type\":\"bytes32[]\",\"internalType\":\"bytes32[]\"}],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"revoke\",\"inputs\":[{\"name\":\"signer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"permissions\",\"type\":\"bytes32[]\",\"internalType\":\"bytes32[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
}

// SessionKeyRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use SessionKeyRegistryMetaData.ABI instead.
var SessionKeyRegistryABI = SessionKeyRegistryMetaData.ABI

// SessionKeyRegistry is an auto generated Go binding around an Ethereum contract.
type SessionKeyRegistry struct {
	SessionKeyRegistryCaller     // Read-only binding to the contract
	SessionKeyRegistryTransactor // Write-only binding to the contract
	SessionKeyRegistryFilterer   // Log filterer for contract events
}

// SessionKeyRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type SessionKeyRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SessionKeyRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type SessionKeyRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SessionKeyRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SessionKeyRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SessionKeyRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SessionKeyRegistrySession struct {
	Contract     *SessionKeyRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts       // Call options to use throughout this session
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// SessionKeyRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SessionKeyRegistryCallerSession struct {
	Contract *SessionKeyRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts             // Call options to use throughout this session
}

// SessionKeyRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SessionKeyRegistryTransactorSession struct {
	Contract     *SessionKeyRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts             // Transaction auth options to use throughout this session
}

// SessionKeyRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type SessionKeyRegistryRaw struct {
	Contract *SessionKeyRegistry // Generic contract binding to access the raw methods on
}

// SessionKeyRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SessionKeyRegistryCallerRaw struct {
	Contract *SessionKeyRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// SessionKeyRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SessionKeyRegistryTransactorRaw struct {
	Contract *SessionKeyRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewSessionKeyRegistry creates a new instance of SessionKeyRegistry, bound to a specific deployed contract.
func NewSessionKeyRegistry(address common.Address, backend bind.ContractBackend) (*SessionKeyRegistry, error) {
	contract, err := bindSessionKeyRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &SessionKeyRegistry{SessionKeyRegistryCaller: SessionKeyRegistryCaller{contract: contract}, SessionKeyRegistryTransactor: SessionKeyRegistryTransactor{contract: contract}, SessionKeyRegistryFilterer: SessionKeyRegistryFilterer{contract: contract}}, nil
}

// NewSessionKeyRegistryCaller creates a new read-only instance of SessionKeyRegistry, bound to a specific deployed contract.
func NewSessionKeyRegistryCaller(address common.Address, caller bind.ContractCaller) (*SessionKeyRegistryCaller, error) {
	contract, err := bindSessionKeyRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SessionKeyRegistryCaller{contract: contract}, nil
}

// NewSessionKeyRegistryTransactor creates a new write-only instance of SessionKeyRegistry, bound to a specific deployed contract.
func NewSessionKeyRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*SessionKeyRegistryTransactor, error) {
	contract, err := bindSessionKeyRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SessionKeyRegistryTransactor{contract: contract}, nil
}

// NewSessionKeyRegistryFilterer creates a new log filterer instance of SessionKeyRegistry, bound to a specific deployed contract.
func NewSessionKeyRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*SessionKeyRegistryFilterer, error) {
	contract, err := bindSessionKeyRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SessionKeyRegistryFilterer{contract: contract}, nil
}

// bindSessionKeyRegistry binds a generic wrapper to an already deployed contract.
func bindSessionKeyRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := SessionKeyRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SessionKeyRegistry *SessionKeyRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SessionKeyRegistry.Contract.SessionKeyRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SessionKeyRegistry *SessionKeyRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.SessionKeyRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SessionKeyRegistry *SessionKeyRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.SessionKeyRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SessionKeyRegistry *SessionKeyRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SessionKeyRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SessionKeyRegistry *SessionKeyRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SessionKeyRegistry *SessionKeyRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.contract.Transact(opts, method, params...)
}

// AuthorizationExpiry is a free data retrieval call binding the contract method 0x9501b2cc.
//
// Solidity: function authorizationExpiry(address user, address signer, bytes32 permission) view returns(uint256)
func (_SessionKeyRegistry *SessionKeyRegistryCaller) AuthorizationExpiry(opts *bind.CallOpts, user common.Address, signer common.Address, permission [32]byte) (*big.Int, error) {
	var out []interface{}
	err := _SessionKeyRegistry.contract.Call(opts, &out, "authorizationExpiry", user, signer, permission)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// AuthorizationExpiry is a free data retrieval call binding the contract method 0x9501b2cc.
//
// Solidity: function authorizationExpiry(address user, address signer, bytes32 permission) view returns(uint256)
func (_SessionKeyRegistry *SessionKeyRegistrySession) AuthorizationExpiry(user common.Address, signer common.Address, permission [32]byte) (*big.Int, error) {
	return _SessionKeyRegistry.Contract.AuthorizationExpiry(&_SessionKeyRegistry.CallOpts, user, signer, permission)
}

// AuthorizationExpiry is a free data retrieval call binding the contract method 0x9501b2cc.
//
// Solidity: function authorizationExpiry(address user, address signer, bytes32 permission) view returns(uint256)
func (_SessionKeyRegistry *SessionKeyRegistryCallerSession) AuthorizationExpiry(user common.Address, signer common.Address, permission [32]byte) (*big.Int, error) {
	return _SessionKeyRegistry.Contract.AuthorizationExpiry(&_SessionKeyRegistry.CallOpts, user, signer, permission)
}

// Login is a paid mutator transaction binding the contract method 0x92247ec0.
//
// Solidity: function login(address signer, uint256 expiry, bytes32[] permissions) returns()
func (_SessionKeyRegistry *SessionKeyRegistryTransactor) Login(opts *bind.TransactOpts, signer common.Address, expiry *big.Int, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.contract.Transact(opts, "login", signer, expiry, permissions)
}

// Login is a paid mutator transaction binding the contract method 0x92247ec0.
//
// Solidity: function login(address signer, uint256 expiry, bytes32[] permissions) returns()
func (_SessionKeyRegistry *SessionKeyRegistrySession) Login(signer common.Address, expiry *big.Int, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.Login(&_SessionKeyRegistry.TransactOpts, signer, expiry, permissions)
}

// Login is a paid mutator transaction binding the contract method 0x92247ec0.
//
// Solidity: function login(address signer, uint256 expiry, bytes32[] permissions) returns()
func (_SessionKeyRegistry *SessionKeyRegistryTransactorSession) Login(signer common.Address, expiry *big.Int, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.Login(&_SessionKeyRegistry.TransactOpts, signer, expiry, permissions)
}

// LoginAndFund is a paid mutator transaction binding the contract method 0x2b894ebc.
//
// Solidity: function loginAndFund(address signer, uint256 expiry, bytes32[] permissions) payable returns()
func (_SessionKeyRegistry *SessionKeyRegistryTransactor) LoginAndFund(opts *bind.TransactOpts, signer common.Address, expiry *big.Int, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.contract.Transact(opts, "loginAndFund", signer, expiry, permissions)
}

// LoginAndFund is a paid mutator transaction binding the contract method 0x2b894ebc.
//
// Solidity: function loginAndFund(address signer, uint256 expiry, bytes32[] permissions) payable returns()
func (_SessionKeyRegistry *SessionKeyRegistrySession) LoginAndFund(signer common.Address, expiry *big.Int, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.LoginAndFund(&_SessionKeyRegistry.TransactOpts, signer, expiry, permissions)
}

// LoginAndFund is a paid mutator transaction binding the contract method 0x2b894ebc.
//
// Solidity: function loginAndFund(address signer, uint256 expiry, bytes32[] permissions) payable returns()
func (_SessionKeyRegistry *SessionKeyRegistryTransactorSession) LoginAndFund(signer common.Address, expiry *big.Int, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.LoginAndFund(&_SessionKeyRegistry.TransactOpts, signer, expiry, permissions)
}

// Revoke is a paid mutator transaction binding the contract method 0xd05989e8.
//
// Solidity: function revoke(address signer, bytes32[] permissions) returns()
func (_SessionKeyRegistry *SessionKeyRegistryTransactor) Revoke(opts *bind.TransactOpts, signer common.Address, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.contract.Transact(opts, "revoke", signer, permissions)
}

// Revoke is a paid mutator transaction binding the contract method 0xd05989e8.
//
// Solidity: function revoke(address signer, bytes32[] permissions) returns()
func (_SessionKeyRegistry *SessionKeyRegistrySession) Revoke(signer common.Address, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.Revoke(&_SessionKeyRegistry.TransactOpts, signer, permissions)
}

// Revoke is a paid mutator transaction binding the contract method 0xd05989e8.
//
// Solidity: function revoke(address signer, bytes32[] permissions) returns()
func (_SessionKeyRegistry *SessionKeyRegistryTransactorSession) Revoke(signer common.Address, permissions [][32]byte) (*types.Transaction, error) {
	return _SessionKeyRegistry.Contract.Revoke(&_SessionKeyRegistry.TransactOpts, signer, permissions)
}
