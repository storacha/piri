package eip712

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// TODO: review this impl in comparison with https://github.com/storyicon/sigverify/blob/main/eip712.go

const extraDataABI = `[{
	"name": "encodeExtraData",
	"type": "function",
	"inputs": [
		{"name": "payer", "type": "address"},
		{"name": "keys", "type": "string[]"},
		{"name": "values", "type": "string[]"},
		{"name": "signature", "type": "bytes"}
	]
}]`

// EncodeExtraDataForDataSetCreated encodes the parameters into the extraData format
// expected by the FilecoinWarmStorageService contract's dataSetCreated function
func EncodeExtraDataForDataSetCreated(payer common.Address, metadataKeys []string, metadataValues []string, signature []byte) ([]byte, error) {
	if len(metadataKeys) != len(metadataValues) {
		return nil, fmt.Errorf("metadata keys and values must have the same length")
	}

	// Parse the ABI
	parsedABI, err := abi.JSON(strings.NewReader(extraDataABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Pack the data
	// Note: We're not actually calling a function, just using ABI encoding
	// So we pack just the arguments without the function selector
	packed, err := parsedABI.Methods["encodeExtraData"].Inputs.Pack(
		payer,
		metadataKeys,
		metadataValues,
		signature,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pack extraData: %w", err)
	}

	return packed, nil
}

// CreateDataSetExtraData creates the complete extraData for dataSetCreated function
// including the EIP-712 signature.
//
// IMPORTANT: The clientDataSetId parameter must match the counter maintained by the
// FilecoinWarmStorageService contract for your payer address. This starts at 0 for
// your first dataset and increments with each dataset you create. You must track
// this counter locally to ensure your signatures are valid.
//
// Example usage:
//   - First dataset:  clientDataSetId = 0
//   - Second dataset: clientDataSetId = 1
//   - Third dataset:  clientDataSetId = 2
func CreateDataSetExtraData(signer *Signer, clientDataSetId *big.Int, payee common.Address, metadata []MetadataEntry) ([]byte, error) {
	// Sign the data
	authSig, err := signer.SignCreateDataSet(clientDataSetId, payee, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to sign CreateDataSet: %w", err)
	}

	// Extract keys and values from metadata
	keys := make([]string, len(metadata))
	values := make([]string, len(metadata))
	for i, entry := range metadata {
		keys[i] = entry.Key
		values[i] = entry.Value
	}

	// Encode the extraData with payer as the signer's address
	extraData, err := EncodeExtraDataForDataSetCreated(
		signer.GetAddress(),
		keys,
		values,
		authSig.Signature,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to encode extraData: %w", err)
	}

	return extraData, nil
}

// HashTypedData is used to calculate the hash of EIP-712 conformant typed data
// hash = keccak256("\x19${byteVersion}${domainSeparator}${hashStruct(message)}")
func HashTypedData(data apitypes.TypedData) ([]byte, []byte, error) {
	domainSeparator, err := data.HashStruct("EIP712Domain", data.Domain.Map())
	if err != nil {
		return nil, nil, err
	}
	dataHash, err := data.HashStruct(data.PrimaryType, data.Message)
	if err != nil {
		return nil, nil, err
	}
	prefixedData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(dataHash)))
	prefixedDataHash := crypto.Keccak256(prefixedData)
	return dataHash, prefixedDataHash, nil
}
