package inprocess

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/signing-service/types"
)

// Signer implements types.SigningService using eip712.Signer directly.
// This provides an in-process implementation that bypasses HTTP and network calls,
// useful for testing and development.
type Signer struct {
	signer *eip712.Signer
}

// Verify that Signer implements types.SigningService at compile time
var _ types.SigningService = (*Signer)(nil)

// New creates a new in-process signing service
func New(signer *eip712.Signer) *Signer {
	return &Signer{signer: signer}
}

// SignCreateDataSet signs a CreateDataSet operation directly
func (s *Signer) SignCreateDataSet(ctx context.Context,
	clientDataSetId *big.Int,
	payee common.Address,
	metadata []eip712.MetadataEntry) (*eip712.AuthSignature, error) {
	// Context is accepted but not used since signing is synchronous
	return s.signer.SignCreateDataSet(clientDataSetId, payee, metadata)
}

// SignAddPieces signs an AddPieces operation directly
func (s *Signer) SignAddPieces(ctx context.Context,
	clientDataSetId *big.Int,
	firstAdded *big.Int,
	pieceData [][]byte,
	metadata [][]eip712.MetadataEntry) (*eip712.AuthSignature, error) {
	return s.signer.SignAddPieces(clientDataSetId, firstAdded, pieceData, metadata)
}

// SignSchedulePieceRemovals signs a SchedulePieceRemovals operation directly
func (s *Signer) SignSchedulePieceRemovals(ctx context.Context,
	clientDataSetId *big.Int,
	pieceIds []*big.Int) (*eip712.AuthSignature, error) {
	return s.signer.SignSchedulePieceRemovals(clientDataSetId, pieceIds)
}

// SignDeleteDataSet signs a DeleteDataSet operation directly
func (s *Signer) SignDeleteDataSet(ctx context.Context,
	clientDataSetId *big.Int) (*eip712.AuthSignature, error) {
	return s.signer.SignDeleteDataSet(clientDataSetId)
}
