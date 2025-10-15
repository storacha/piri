package smartcontracts

import (
	"github.com/storacha/filecoin-services/go/bindings"
)

// Type aliases for types used in public interfaces
// These types are generated from the smart contract ABIs
// and are exposed here to avoid direct dependency on internal packages

// IPDPTypesProof represents a merkle proof for PDP verification
type IPDPTypesProof = bindings.IPDPTypesProof

// IPDPTypesPieceIdAndOffset represents a piece ID and its offset within a dataset
type IPDPTypesPieceIdAndOffset = bindings.IPDPTypesPieceIdAndOffset

// CidsCid represents a CID (Content Identifier) as used in the smart contracts
type CidsCid = bindings.CidsCid
