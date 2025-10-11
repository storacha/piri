package eip712

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// ERC20 ABI for querying token name, balance, and decimals
const erc20ABI = `[
	{"constant":true,"inputs":[],"name":"name","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},
	{"constant":true,"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},
	{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}
]`

// EIP2612 ABI for querying version and nonces
const eip2612ABI = `[
	{"constant":true,"inputs":[],"name":"version","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},
	{"constant":true,"inputs":[{"name":"owner","type":"address"}],"name":"nonces","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}
]`

// TokenInfo contains the domain information needed for permit signing
type TokenInfo struct {
	Name    string
	Version string
	Nonce   *big.Int
}

// QueryTokenInfo queries the token contract for name, version, and nonce
// Returns version as "1" if the token doesn't implement the version() function
func QueryTokenInfo(ctx context.Context, client *ethclient.Client, tokenAddress common.Address, owner common.Address) (*TokenInfo, error) {
	// Parse ABIs
	erc20Parsed, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 ABI: %w", err)
	}

	eip2612Parsed, err := abi.JSON(strings.NewReader(eip2612ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse EIP2612 ABI: %w", err)
	}

	// Query name
	nameData, err := erc20Parsed.Pack("name")
	if err != nil {
		return nil, fmt.Errorf("failed to pack name call: %w", err)
	}

	nameResult, err := client.CallContract(ctx, callMsg(tokenAddress, nameData), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call name(): %w", err)
	}

	var name string
	err = erc20Parsed.UnpackIntoInterface(&name, "name", nameResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack name: %w", err)
	}

	// Query version (with fallback to "1")
	version := "1" // Default fallback
	versionData, err := eip2612Parsed.Pack("version")
	if err == nil {
		versionResult, err := client.CallContract(ctx, callMsg(tokenAddress, versionData), nil)
		if err == nil {
			var v string
			err = eip2612Parsed.UnpackIntoInterface(&v, "version", versionResult)
			if err == nil && len(v) > 0 {
				version = v
			}
		}
	}

	// Query nonce
	nonceData, err := eip2612Parsed.Pack("nonces", owner)
	if err != nil {
		return nil, fmt.Errorf("failed to pack nonces call: %w", err)
	}

	nonceResult, err := client.CallContract(ctx, callMsg(tokenAddress, nonceData), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call nonces(): %w (token may not support EIP-2612)", err)
	}

	var nonce *big.Int
	err = eip2612Parsed.UnpackIntoInterface(&nonce, "nonces", nonceResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack nonce: %w", err)
	}

	return &TokenInfo{
		Name:    name,
		Version: version,
		Nonce:   nonce,
	}, nil
}

// GeneratePermitSignature creates an EIP-2612 permit signature for token approval
// tokenAddress: the ERC20 token contract address (used as verifyingContract in domain)
// spender: the address being approved to spend tokens (typically Payments contract)
// amount: the amount to approve
// deadline: unix timestamp when permit expires
// privateKey: the signer's private key
// chainID: the chain ID for the domain
func GeneratePermitSignature(
	ctx context.Context,
	client *ethclient.Client,
	tokenAddress common.Address,
	spender common.Address,
	amount *big.Int,
	deadline *big.Int,
	privateKey *ecdsa.PrivateKey,
	chainID *big.Int,
) (*PermitSignature, error) {
	// Derive owner address from private key
	owner := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Query token for permit domain information
	tokenInfo, err := QueryTokenInfo(ctx, client, tokenAddress, owner)
	if err != nil {
		return nil, fmt.Errorf("failed to query token info: %w", err)
	}

	// Build EIP-712 domain (uses token address as verifyingContract)
	domain := apitypes.TypedDataDomain{
		Name:              tokenInfo.Name,
		Version:           tokenInfo.Version,
		ChainId:           (*math.HexOrDecimal256)(chainID),
		VerifyingContract: tokenAddress.Hex(),
	}

	// Create permit message
	message := map[string]interface{}{
		"owner":    owner.Hex(),
		"spender":  spender.Hex(),
		"value":    amount,
		"nonce":    tokenInfo.Nonce,
		"deadline": deadline,
	}

	// Get the EIP-712 hash to sign
	hash, err := getPermitHash(domain, message)
	if err != nil {
		return nil, fmt.Errorf("failed to get permit hash: %w", err)
	}

	// Sign the hash
	signature, err := crypto.Sign(hash, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	// Transform V from recovery ID to Ethereum signature standard
	// Ethereum uses 27/28, crypto.Sign returns 0/1
	v := signature[64] + 27

	// Extract r and s
	var r, s [32]byte
	copy(r[:], signature[:32])
	copy(s[:], signature[32:64])

	return &PermitSignature{
		V:        v,
		R:        r,
		S:        s,
		Deadline: deadline,
	}, nil
}

// GetTokenBalance queries the ERC20 token balance for an account
func GetTokenBalance(ctx context.Context, client *ethclient.Client, tokenAddress common.Address, account common.Address) (*big.Int, error) {
	// Parse ERC20 ABI
	erc20Parsed, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 ABI: %w", err)
	}

	// Pack balanceOf call
	balanceData, err := erc20Parsed.Pack("balanceOf", account)
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf call: %w", err)
	}

	// Call contract
	balanceResult, err := client.CallContract(ctx, callMsg(tokenAddress, balanceData), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call balanceOf(): %w", err)
	}

	// Unpack result
	var balance *big.Int
	err = erc20Parsed.UnpackIntoInterface(&balance, "balanceOf", balanceResult)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack balance: %w", err)
	}

	return balance, nil
}

// GetTokenDecimals queries the ERC20 token decimals
func GetTokenDecimals(ctx context.Context, client *ethclient.Client, tokenAddress common.Address) (uint8, error) {
	// Parse ERC20 ABI
	erc20Parsed, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return 0, fmt.Errorf("failed to parse ERC20 ABI: %w", err)
	}

	// Pack decimals call
	decimalsData, err := erc20Parsed.Pack("decimals")
	if err != nil {
		return 0, fmt.Errorf("failed to pack decimals call: %w", err)
	}

	// Call contract
	decimalsResult, err := client.CallContract(ctx, callMsg(tokenAddress, decimalsData), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call decimals(): %w", err)
	}

	// Unpack result
	var decimals uint8
	err = erc20Parsed.UnpackIntoInterface(&decimals, "decimals", decimalsResult)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack decimals: %w", err)
	}

	return decimals, nil
}

// getPermitHash computes the EIP-712 hash for a permit message
func getPermitHash(domain apitypes.TypedDataDomain, message map[string]interface{}) ([]byte, error) {
	typedData := apitypes.TypedData{
		Types:       EIP2612PermitTypes,
		PrimaryType: "Permit",
		Domain:      domain,
		Message:     message,
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return nil, err
	}

	messageHash, err := typedData.HashStruct("Permit", message)
	if err != nil {
		return nil, err
	}

	// EIP-712: keccak256("\x19\x01" || domainSeparator || messageHash)
	rawData := []byte{0x19, 0x01}
	rawData = append(rawData, domainSeparator...)
	rawData = append(rawData, messageHash...)

	return crypto.Keccak256(rawData), nil
}

// Helper to create ethereum CallMsg
func callMsg(to common.Address, data []byte) ethereum.CallMsg {
	return ethereum.CallMsg{
		To:   &to,
		Data: data,
	}
}
