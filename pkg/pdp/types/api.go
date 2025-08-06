package types

import (
	"context"
	"io"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
)

type ProofSetStatus struct {
	TxHash   common.Hash
	TxStatus string
	Created  bool
	ID       uint64
}

type ProofSet struct {
	ID                     uint64
	Initialized            bool
	NextChallengeEpoch     int64
	PreviousChallengeEpoch int64
	ProvingPeriod          int64
	ChallengeWindow        int64
	Roots                  []RootEntry
}

type RootEntry struct {
	RootID        uint64
	RootCID       cid.Cid
	SubrootCID    cid.Cid
	SubrootOffset int64
}

type RootAdd struct {
	Root     cid.Cid
	SubRoots []cid.Cid
}

type PieceAllocation struct {
	Piece  Piece
	Notify *url.URL
}

type Piece struct {
	// Name of the hash function used
	// sha2-256-trunc254-padded - CommP
	// sha2-256 - Blob sha256
	Name string

	// hex encoded hash
	Hash string

	// Size of the piece in bytes
	Size int64
}

type PieceUpload struct {
	ID   uuid.UUID
	Data io.Reader
}

type AllocatedPiece struct {
	Allocated bool
	Piece     cid.Cid
	UploadID  uuid.UUID
}

type PieceReader struct {
	Size int64
	Data io.ReadCloser
}

type API interface {
	ProofSetAPI
	PieceAPI
}

type ProofSetAPI interface {
	CreateProofSet(ctx context.Context, recordKeeper common.Address) (common.Hash, error)
	GetProofSetStatus(ctx context.Context, txHash common.Hash) (*ProofSetStatus, error)
	GetProofSet(ctx context.Context, proofSetID uint64) (*ProofSet, error)
	AddRoots(ctx context.Context, proofSetID uint64, roots []RootAdd) (common.Hash, error)
	RemoveRoot(ctx context.Context, proofSetID uint64, rootID uint64) (common.Hash, error)
}

type PieceAPI interface {
	AllocatePiece(ctx context.Context, allocation PieceAllocation) (*AllocatedPiece, error)
	UploadPiece(ctx context.Context, upload PieceUpload) error
	FindPiece(ctx context.Context, piece Piece) (cid.Cid, bool, error)
	ReadPiece(ctx context.Context, piece cid.Cid) (*PieceReader, error)
}
