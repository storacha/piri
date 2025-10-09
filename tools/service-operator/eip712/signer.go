package eip712

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type AuthSignature struct {
	Signature  []byte         `json:"signature"`
	V          uint8          `json:"v"`
	R          common.Hash    `json:"r"`
	S          common.Hash    `json:"s"`
	SignedData []byte         `json:"signedData"`
	Signer     common.Address `json:"signer"`
}

// Marshal returns the signature as bytes in the format expected by the smart contract
// The format is: R (32 bytes) + S (32 bytes) + V (1 byte)
func (a *AuthSignature) Marshal() ([]byte, error) {
	if len(a.Signature) == 65 {
		// If we already have the full signature, return it
		return a.Signature, nil
	}

	// Otherwise construct it from R, S, V
	sig := make([]byte, 65)
	copy(sig[0:32], a.R[:])
	copy(sig[32:64], a.S[:])
	sig[64] = a.V
	return sig, nil
}

type Signer struct {
	privateKey        *ecdsa.PrivateKey
	address           common.Address
	chainId           *big.Int
	verifyingContract common.Address
}

func NewSigner(privateKey *ecdsa.PrivateKey, chainId *big.Int, verifyingContract common.Address) *Signer {
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return &Signer{
		privateKey:        privateKey,
		address:           address,
		chainId:           chainId,
		verifyingContract: verifyingContract,
	}
}

func (s *Signer) GetAddress() common.Address {
	return s.address
}

func (s *Signer) SignCreateDataSet(clientDataSetId *big.Int, payee common.Address, metadata []MetadataEntry) (*AuthSignature, error) {
	// Convert metadata to the format expected by apitypes
	metadataArray := make([]map[string]interface{}, len(metadata))
	for i, entry := range metadata {
		metadataArray[i] = map[string]interface{}{
			"key":   entry.Key,
			"value": entry.Value,
		}
	}

	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
		"payee":           strings.ToLower(payee.Hex()),
		"metadata":        metadataArray,
	}

	return s.signTypedData("CreateDataSet", message)
}

// RecoverCreateDataSetSigner recovers the signer address from a CreateDataSet signature
func (s *Signer) RecoverCreateDataSetSigner(clientDataSetId *big.Int, payee common.Address, metadata []MetadataEntry, signature *AuthSignature) (common.Address, error) {
	// Convert metadata to the format expected by apitypes
	metadataArray := make([]map[string]interface{}, len(metadata))
	for i, entry := range metadata {
		metadataArray[i] = map[string]interface{}{
			"key":   entry.Key,
			"value": entry.Value,
		}
	}

	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
		"payee":           strings.ToLower(payee.Hex()),
		"metadata":        metadataArray,
	}

	domain := GetDomain(s.chainId, s.verifyingContract)
	hash, err := GetMessageHash(domain, "CreateDataSet", message)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to get message hash: %w", err)
	}

	// crypto.SigToPub expects v to be 0 or 1, but signature has v as 27 or 28
	// Create a copy with adjusted v for recovery
	recoverySignature := make([]byte, 65)
	copy(recoverySignature, signature.Signature)
	if recoverySignature[64] >= 27 {
		recoverySignature[64] -= 27
	}

	// Recover the public key from the signature
	pubKey, err := crypto.SigToPub(hash, recoverySignature)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to recover public key: %w", err)
	}

	return crypto.PubkeyToAddress(*pubKey), nil
}

func (s *Signer) SignAddPieces(clientDataSetId, firstAdded *big.Int, pieceData [][]byte, metadata [][]MetadataEntry) (*AuthSignature, error) {
	cids := make([]map[string]interface{}, len(pieceData))
	for i, data := range pieceData {
		cids[i] = map[string]interface{}{
			"data": data,
		}
	}

	pieceMetadata := make([]map[string]interface{}, len(metadata))
	for i, meta := range metadata {
		// Convert MetadataEntry array to expected format
		metadataArray := make([]map[string]interface{}, len(meta))
		for j, entry := range meta {
			metadataArray[j] = map[string]interface{}{
				"key":   entry.Key,
				"value": entry.Value,
			}
		}

		pieceMetadata[i] = map[string]interface{}{
			"pieceIndex": big.NewInt(int64(i)),
			"metadata":   metadataArray,
		}
	}

	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
		"firstAdded":      firstAdded,
		"pieceData":       cids,
		"pieceMetadata":   pieceMetadata,
	}

	return s.signTypedData("AddPieces", message)
}

func (s *Signer) SignSchedulePieceRemovals(clientDataSetId *big.Int, pieceIds []*big.Int) (*AuthSignature, error) {
	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
		"pieceIds":        pieceIds,
	}

	return s.signTypedData("SchedulePieceRemovals", message)
}

func (s *Signer) SignDeleteDataSet(clientDataSetId *big.Int) (*AuthSignature, error) {
	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
	}

	return s.signTypedData("DeleteDataSet", message)
}

func (s *Signer) signTypedData(primaryType string, message map[string]interface{}) (*AuthSignature, error) {
	domain := GetDomain(s.chainId, s.verifyingContract)

	fmt.Printf("DEBUG signTypedData: domain verifyingContract = %s\n", s.verifyingContract.Hex())
	fmt.Printf("DEBUG signTypedData: primaryType = %s\n", primaryType)
	fmt.Printf("DEBUG signTypedData: message = %+v\n", message)

	// Get the EIP-712 hash to sign
	hash, err := GetMessageHash(domain, primaryType, message)
	if err != nil {
		return nil, fmt.Errorf("failed to get message hash: %w", err)
	}

	fmt.Printf("DEBUG signTypedData: hash = 0x%x\n", hash)

	// Sign the hash
	signature, err := crypto.Sign(hash, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	// Transform V from recovery ID to Ethereum signature standard
	// Ethereum uses 27/28, crypto.Sign returns 0/1
	v := signature[64] + 27

	// Extract r and s
	r := common.BytesToHash(signature[:32])
	sig_s := common.BytesToHash(signature[32:64])

	// Create full signature with adjusted V
	fullSig := make([]byte, 65)
	copy(fullSig[:32], signature[:32])
	copy(fullSig[32:64], signature[32:64])
	fullSig[64] = v

	return &AuthSignature{
		Signature:  fullSig,
		V:          v,
		R:          r,
		S:          sig_s,
		SignedData: hash,
		Signer:     s.address,
	}, nil
}
