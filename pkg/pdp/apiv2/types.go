package apiv2

import (
	"context"
	"io"
	"net/url"
)

/*
The architecture is:

  ┌─────────────────┐
  │   HTTP Client   │
  └─────────────────┘
           │
           │ HTTP
           ↓
  ┌─────────────────┐    ┌─────────────────┐
  │   HTTP Server   │    │    API          │ ← shared business logic, can be used in process
  └─────────────────┘    └─────────────────┘
           │ uses                 │
           └──────────┬───────────┘
                      ↓
              ┌─────────────────┐
              │  PDPService     │
              └─────────────────┘

This allow alternative implementations to simply take a dependency on the API and run everything in one process.
The server simply wraps the API, interactions then go through the client. Since the Client and API both
implement the PDP interface they may be used interchangeably.
TODO: GetPieceURL is complicated as the method assumes there is an endpoint to join the piece on.
- when operating as two process this works as it expected, the client uses the endpoint it's connected
  to to create a valid URL reference
- when running as a single process this is a bit complicated, implementation now provides the API with an endpoint URL
  corresponding to the service its operating over
*/

// PDP defines the contract for all PDP operations
type PDP interface {
	Ping(ctx context.Context) error
	CreateProofSet(ctx context.Context, request CreateProofSet) (StatusRef, error)
	ProofSetCreationStatus(ctx context.Context, ref StatusRef) (ProofSetStatus, error)
	GetProofSet(ctx context.Context, id uint64) (ProofSet, error)
	DeleteProofSet(ctx context.Context, id uint64) error
	AddRootsToProofSet(ctx context.Context, id uint64, addRoots []AddRootRequest) error
	AddPiece(ctx context.Context, addPiece AddPiece) (*UploadRef, error)
	UploadPiece(ctx context.Context, ref UploadRef, data io.Reader) error
	FindPiece(ctx context.Context, piece PieceHash) (FoundPiece, error)
	GetPiece(ctx context.Context, pieceCid string) (PieceReader, error)
	GetPieceURL(pieceCid string) url.URL
}

// Shared types used by both client and server

type AddRootsPayload struct {
	Roots     []AddRootRequest `json:"roots"`
	ExtraData *string          `json:"extraData,omitempty"`
}

type AddRootRequest struct {
	RootCID  string         `json:"rootCid"`
	Subroots []SubrootEntry `json:"subroots"`
}

type SubrootEntry struct {
	SubrootCID string `json:"subrootCid"`
}

type CreateProofSet struct {
	RecordKeeper string `json:"recordKeeper"`
}

type StatusRef struct {
	URL string
}

type ProofSetStatus struct {
	CreateMessageHash string  `json:"createMessageHash"`
	ProofsetCreated   bool    `json:"proofsetCreated"`
	Service           string  `json:"service"`
	TxStatus          string  `json:"txStatus"`
	OK                *bool   `json:"ok"`
	ProofSetId        *uint64 `json:"proofSetId,omitempty"`
}

type ProofSet struct {
	ID                 uint64      `json:"id"`
	NextChallengeEpoch *int64      `json:"nextChallengeEpoch"`
	Roots              []RootEntry `json:"roots"`
}

type RootEntry struct {
	RootID        uint64 `json:"rootId"`
	RootCID       string `json:"rootCid"`
	SubrootCID    string `json:"subrootCid"`
	SubrootOffset int64  `json:"subrootOffset"`
}

type AddPiece struct {
	Check  PieceHash `json:"check"`
	Notify string    `json:"notify,omitempty"`
}

type PieceHash struct {
	// Name of the hash function used
	// sha2-256-trunc254-padded - CommP
	// sha2-256 - Blob sha256
	Name string `json:"name"`

	// hex encoded hash
	Hash string `json:"hash"`

	// Size of the piece in bytes
	Size int64 `json:"size"`
}

type UploadRef struct {
	URL string
}

type FoundPiece struct {
	PieceCID string `json:"piece_cid"`
}

type PieceReader struct {
	Data io.ReadCloser
	Size int64
}
