package types

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/piri/tools/service-operator/eip712"
)

// CreateDataSetRequest represents the request payload for creating a dataset
type CreateDataSetRequest struct {
	ClientDataSetId string                 `json:"clientDataSetId"`
	Payee           string                 `json:"payee"`
	Metadata        []eip712.MetadataEntry `json:"metadata"`
}

// AddPiecesRequest represents the request payload for adding pieces
type AddPiecesRequest struct {
	ClientDataSetId string                   `json:"clientDataSetId"`
	FirstAdded      string                   `json:"firstAdded"`
	PieceData       []string                 `json:"pieceData"` // hex-encoded bytes
	Metadata        [][]eip712.MetadataEntry `json:"metadata"`
}

// SchedulePieceRemovalsRequest represents the request payload for scheduling piece removals
type SchedulePieceRemovalsRequest struct {
	ClientDataSetId string   `json:"clientDataSetId"`
	PieceIds        []string `json:"pieceIds"`
}

// DeleteDataSetRequest represents the request payload for deleting a dataset
type DeleteDataSetRequest struct {
	ClientDataSetId string `json:"clientDataSetId"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status string `json:"status"`
	Signer string `json:"signer"`
}

// SigningService defines the interface for PDP operation signing.
// This can be implemented by:
// - HTTP client (remote signing service)
// - In-process signer (direct eip712.Signer)
//
// This allows piri nodes to use either implementation interchangeably,
// enabling easy testing and development without running a separate service.
type SigningService interface {
	// SignCreateDataSet signs a CreateDataSet operation
	SignCreateDataSet(ctx context.Context,
		clientDataSetId *big.Int,
		payee common.Address,
		metadata []eip712.MetadataEntry) (*eip712.AuthSignature, error)

	// SignAddPieces signs an AddPieces operation
	SignAddPieces(ctx context.Context,
		clientDataSetId *big.Int,
		firstAdded *big.Int,
		pieceData [][]byte,
		metadata [][]eip712.MetadataEntry) (*eip712.AuthSignature, error)

	// SignSchedulePieceRemovals signs a SchedulePieceRemovals operation
	SignSchedulePieceRemovals(ctx context.Context,
		clientDataSetId *big.Int,
		pieceIds []*big.Int) (*eip712.AuthSignature, error)

	// SignDeleteDataSet signs a DeleteDataSet operation
	SignDeleteDataSet(ctx context.Context,
		clientDataSetId *big.Int) (*eip712.AuthSignature, error)
}
