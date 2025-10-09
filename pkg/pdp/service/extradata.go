package service

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/tools/service-operator/eip712"
)

// ExtraDataEncoder provides functions to encode extraData for PDP operations
// that require EIP-712 signatures for the FilecoinWarmStorageService contract
type ExtraDataEncoder struct{}

// NewExtraDataEncoder creates a new ExtraDataEncoder
func NewExtraDataEncoder() *ExtraDataEncoder {
	return &ExtraDataEncoder{}
}

// EncodeCreateDataSetExtraData encodes the extraData for dataSetCreated callback
// The format is: abi.encode(address payer, string[] metadataKeys, string[] metadataValues, bytes signature)
func (e *ExtraDataEncoder) EncodeCreateDataSetExtraData(
	payer common.Address,
	metadata []eip712.MetadataEntry,
	signature *eip712.AuthSignature,
) ([]byte, error) {
	// Split metadata into keys and values arrays
	keys := make([]string, len(metadata))
	values := make([]string, len(metadata))
	for i, m := range metadata {
		keys[i] = m.Key
		values[i] = m.Value
	}

	// Marshal the signature
	signatureBytes, err := signature.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signature: %w", err)
	}

	// Define the ABI for encoding
	// We need to pack: (address, string[], string[], bytes)
	abiDef := `[{"type":"function","name":"encode","inputs":[
		{"name":"payer","type":"address"},
		{"name":"metadataKeys","type":"string[]"},
		{"name":"metadataValues","type":"string[]"},
		{"name":"signature","type":"bytes"}
	]}]`

	parsedABI, err := abi.JSON(strings.NewReader(abiDef))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Pack the data
	packed, err := parsedABI.Pack("encode", payer, keys, values, signatureBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to pack extraData: %w", err)
	}

	// Remove the function selector (first 4 bytes)
	if len(packed) < 4 {
		return nil, fmt.Errorf("packed data too short")
	}
	return packed[4:], nil
}

// EncodeAddPiecesExtraData encodes the extraData for piecesAdded callback
// The format is: abi.encode(bytes signature, string[][] metadataKeys, string[][] metadataValues)
func (e *ExtraDataEncoder) EncodeAddPiecesExtraData(
	signature *eip712.AuthSignature,
	metadata [][]eip712.MetadataEntry,
) ([]byte, error) {
	// Marshal the signature
	signatureBytes, err := signature.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signature: %w", err)
	}

	// Split metadata into keys and values arrays
	keysArray := make([][]string, len(metadata))
	valuesArray := make([][]string, len(metadata))
	for i, pieceMetadata := range metadata {
		keys := make([]string, len(pieceMetadata))
		values := make([]string, len(pieceMetadata))
		for j, m := range pieceMetadata {
			keys[j] = m.Key
			values[j] = m.Value
		}
		keysArray[i] = keys
		valuesArray[i] = values
	}

	// Define the ABI for encoding
	abiDef := `[{"type":"function","name":"encode","inputs":[
		{"name":"signature","type":"bytes"},
		{"name":"metadataKeys","type":"string[][]"},
		{"name":"metadataValues","type":"string[][]"}
	]}]`

	parsedABI, err := abi.JSON(strings.NewReader(abiDef))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Pack the data
	packed, err := parsedABI.Pack("encode", signatureBytes, keysArray, valuesArray)
	if err != nil {
		return nil, fmt.Errorf("failed to pack extraData: %w", err)
	}

	// Remove the function selector (first 4 bytes)
	if len(packed) < 4 {
		return nil, fmt.Errorf("packed data too short")
	}
	return packed[4:], nil
}

// EncodeSchedulePieceRemovalsExtraData encodes the extraData for piecesScheduledRemove callback
// The format is: abi.encode(bytes signature)
func (e *ExtraDataEncoder) EncodeSchedulePieceRemovalsExtraData(
	signature *eip712.AuthSignature,
) ([]byte, error) {
	// Marshal the signature
	signatureBytes, err := signature.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signature: %w", err)
	}

	// Define the ABI for encoding
	abiDef := `[{"type":"function","name":"encode","inputs":[
		{"name":"signature","type":"bytes"}
	]}]`

	parsedABI, err := abi.JSON(strings.NewReader(abiDef))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Pack the data
	packed, err := parsedABI.Pack("encode", signatureBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to pack extraData: %w", err)
	}

	// Remove the function selector (first 4 bytes)
	if len(packed) < 4 {
		return nil, fmt.Errorf("packed data too short")
	}
	return packed[4:], nil
}

// EncodeDeleteDataSetExtraData encodes the extraData for dataSetDeleted callback
// The format is: abi.encode(bytes signature)
func (e *ExtraDataEncoder) EncodeDeleteDataSetExtraData(
	signature *eip712.AuthSignature,
) ([]byte, error) {
	// This is the same as SchedulePieceRemovals - just a signature
	return e.EncodeSchedulePieceRemovalsExtraData(signature)
}

// ParseMetadataEntries converts a slice of key=value strings to MetadataEntry slice
// This is a helper for parsing metadata from command line or configuration
func ParseMetadataEntries(entries []string) ([]eip712.MetadataEntry, error) {
	metadata := make([]eip712.MetadataEntry, 0, len(entries))
	for _, entry := range entries {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid metadata entry format: %s (expected key=value)", entry)
		}
		metadata = append(metadata, eip712.MetadataEntry{
			Key:   parts[0],
			Value: parts[1],
		})
	}
	return metadata, nil
}

// MetadataToStringSlices converts MetadataEntry slice to separate key and value slices
// This is useful when calling smart contract methods that expect separate arrays
func MetadataToStringSlices(metadata []eip712.MetadataEntry) (keys []string, values []string) {
	keys = make([]string, len(metadata))
	values = make([]string, len(metadata))
	for i, m := range metadata {
		keys[i] = m.Key
		values[i] = m.Value
	}
	return keys, values
}