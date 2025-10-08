package eip712

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	EIP712DomainName    = "FilecoinWarmStorageService"
	EIP712DomainVersion = "1"
)

type MetadataEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type CreateDataSet struct {
	ClientDataSetId *big.Int        `json:"clientDataSetId"`
	Payee           common.Address  `json:"payee"`
	Metadata        []MetadataEntry `json:"metadata"`
}

type Cid struct {
	Data []byte `json:"data"`
}

type PieceMetadata struct {
	PieceIndex *big.Int        `json:"pieceIndex"`
	Metadata   []MetadataEntry `json:"metadata"`
}

type AddPieces struct {
	ClientDataSetId *big.Int        `json:"clientDataSetId"`
	FirstAdded      *big.Int        `json:"firstAdded"`
	PieceData       []Cid           `json:"pieceData"`
	PieceMetadata   []PieceMetadata `json:"pieceMetadata"`
}

type SchedulePieceRemovals struct {
	ClientDataSetId *big.Int   `json:"clientDataSetId"`
	PieceIds        []*big.Int `json:"pieceIds"`
}

type DeleteDataSet struct {
	ClientDataSetId *big.Int `json:"clientDataSetId"`
}

// EIP712Types defines the type definitions for PDP auth operations
var EIP712Types = apitypes.Types{
	"EIP712Domain": {
		{Name: "name", Type: "string"},
		{Name: "version", Type: "string"},
		{Name: "chainId", Type: "uint256"},
		{Name: "verifyingContract", Type: "address"},
	},
	"MetadataEntry": {
		{Name: "key", Type: "string"},
		{Name: "value", Type: "string"},
	},
	"CreateDataSet": {
		{Name: "clientDataSetId", Type: "uint256"},
		{Name: "payee", Type: "address"},
		{Name: "metadata", Type: "MetadataEntry[]"},
	},
	"Cid": {
		{Name: "data", Type: "bytes"},
	},
	"PieceMetadata": {
		{Name: "pieceIndex", Type: "uint256"},
		{Name: "metadata", Type: "MetadataEntry[]"},
	},
	"AddPieces": {
		{Name: "clientDataSetId", Type: "uint256"},
		{Name: "firstAdded", Type: "uint256"},
		{Name: "pieceData", Type: "Cid[]"},
		{Name: "pieceMetadata", Type: "PieceMetadata[]"},
	},
	"SchedulePieceRemovals": {
		{Name: "clientDataSetId", Type: "uint256"},
		{Name: "pieceIds", Type: "uint256[]"},
	},
	"DeleteDataSet": {
		{Name: "clientDataSetId", Type: "uint256"},
	},
}

// EIP2612PermitTypes defines the type definitions for EIP-2612 token permits
var EIP2612PermitTypes = apitypes.Types{
	"EIP712Domain": {
		{Name: "name", Type: "string"},
		{Name: "version", Type: "string"},
		{Name: "chainId", Type: "uint256"},
		{Name: "verifyingContract", Type: "address"},
	},
	"Permit": {
		{Name: "owner", Type: "address"},
		{Name: "spender", Type: "address"},
		{Name: "value", Type: "uint256"},
		{Name: "nonce", Type: "uint256"},
		{Name: "deadline", Type: "uint256"},
	},
}

// PermitSignature contains the signature and components for an EIP-2612 permit
type PermitSignature struct {
	V        uint8
	R        [32]byte
	S        [32]byte
	Deadline *big.Int
}

func GetDomain(chainId *big.Int, verifyingContract common.Address) apitypes.TypedDataDomain {
	return apitypes.TypedDataDomain{
		Name:              EIP712DomainName,
		Version:           EIP712DomainVersion,
		ChainId:           (*math.HexOrDecimal256)(chainId),
		VerifyingContract: verifyingContract.Hex(),
	}
}

func HashStruct(primaryType string, data interface{}) ([]byte, error) {
	message, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data must be map[string]interface{}")
	}

	typedData := apitypes.TypedData{
		Types:       EIP712Types,
		PrimaryType: primaryType,
		Message:     message,
	}

	return typedData.HashStruct(primaryType, message)
}

func GetMessageHash(domain apitypes.TypedDataDomain, primaryType string, message interface{}) ([]byte, error) {
	messageMap, ok := message.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("message must be map[string]interface{}")
	}

	typedData := apitypes.TypedData{
		Types:       EIP712Types,
		PrimaryType: primaryType,
		Domain:      domain,
		Message:     messageMap,
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return nil, err
	}

	messageHash, err := typedData.HashStruct(primaryType, messageMap)
	if err != nil {
		return nil, err
	}

	rawData := []byte{0x19, 0x01}
	rawData = append(rawData, domainSeparator...)
	rawData = append(rawData, messageHash...)

	return crypto.Keccak256(rawData), nil
}
