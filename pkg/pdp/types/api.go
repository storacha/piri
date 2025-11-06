package types

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
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

type ProofSetState struct {
	ID uint64
	// if the proof set has been initialized with a root, and is expecting proofs to be submitted.
	Initialized bool
	// When the next challenge for a proof will be issued
	NextChallengeEpoch int64
	// When the last challenge for a proof was issued
	PreviousChallengeEpoch int64
	// The proving period of this proof set
	ProvingPeriod int64
	// The challenge window of this proof set
	ChallengeWindow int64
	// The current epoch of the chain
	CurrentEpoch int64
	// true if a challenge has been issued: CurrentEpoch >= NextChallengeEpoch
	ChallengedIssued bool
	// true if in challenge window: CurrentEpoch < NextChallengeEpoch + ChallengeWindow
	InChallengeWindow bool
	// true if we missed the challenge: CurrentEpoch > NextChallengeEpoch + ChallengeWindow
	IsInFaultState bool
	// true if we submitted a proof for the current ChallengeWindow
	HasProven bool
	// true if the node is currently generating a proof
	IsProving bool

	// The state of the proof set present in the contract
	ContractState ProofSetContractState
}

type ProofSetContractState struct {
	// owners of the proof set
	Owners []common.Address
	// The start of the NEXT OPEN proving period's challenge window
	NextChallengeWindowStart uint64
	// the epoch of the next challenge
	NextChallengeEpoch uint64
	// Max number of epochs between two consecutive proofs
	MaxProvingPeriod uint64
	// challengeWindow Number of epochs for the challenge window
	ChallengeWindow uint64
	//index of the most recently added leaf that is challengeable in the current proving period
	ChallengeRange uint64
	// piece ids of the pieces scheduled for removal at the start of the next proving period
	ScheduledRemovals []uint64
	// estimated cost of submitting a proof
	ProofFee uint64
	// estimated cost of submitting a proof with buffer applied
	ProofFeeBuffered uint64
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

func (p Piece) Multihash() (multihash.Multihash, error) {
	_, ok := multihash.Names[p.Name]
	if !ok {
		return nil, fmt.Errorf("unknown piece multihash: %s", p.Name)
	}

	hashBytes, err := hex.DecodeString(p.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode piece %s:%s: %w", p.Name, p.Hash, err)
	}

	return multihash.EncodeName(hashBytes, p.Name)
}

type PieceUpload struct {
	ID   uuid.UUID
	Data io.Reader
}

type API interface {
	ProofSetAPI
	PieceAPI
	ProviderAPI
}

type AllocatedPiece struct {
	Allocated bool
	Piece     cid.Cid
	UploadID  uuid.UUID
}

type PieceReader struct {
	// Size is the total size of the piece
	Size int64
	Data io.ReadCloser
}

type RegisterProviderParams struct {
	Name        string
	Description string
}

type RegisterProviderResults struct {
	// transaction hash of message sent by provider to register, when set all
	// other fields are empty
	TransactionHash common.Hash
	// address of the provider
	Address common.Address
	// address the provider will receive payment on
	Payee common.Address
	// ID of provider
	ID uint64
	// True if the provider is registered (don't imply they have been approved
	// the service contract.
	IsActive bool
	// Optional name chosen by provider
	Name string
	// Optional description chosen by provider
	Description string
}

type GetProviderStatusResults struct {
	// ID of provider (0 if not registered)
	ID uint64
	// address of the provider
	Address common.Address
	// address the provider will receive payment on
	Payee common.Address
	// True if the provider is registered in the registry
	IsRegistered bool
	// True if the provider is active
	IsActive bool
	// Optional name chosen by provider
	Name string
	// Optional description chosen by provider
	Description string
	// Registration status: "not_registered", "pending", or "registered"
	RegistrationStatus string
	// True if contract operator approved provider to operate
	IsApproved bool
}

const (
	ProductTypePDP uint8 = 0
	// TODO we need to generate type for this from the contract ABI
	// this is based on the contract code, right now there is only a single product type
	// as an enum, so it's value is 0
)

type ProofSetAPI interface {
	CreateProofSet(ctx context.Context) (common.Hash, error)
	GetProofSetStatus(ctx context.Context, txHash common.Hash) (*ProofSetStatus, error)
	GetProofSet(ctx context.Context, proofSetID uint64) (*ProofSet, error)
	AddRoots(ctx context.Context, proofSetID uint64, roots []RootAdd) (common.Hash, error)
	RemoveRoot(ctx context.Context, proofSetID uint64, rootID uint64) (common.Hash, error)
}

type Range struct {
	// Start is the byte to start extracting from (inclusive).
	Start uint64
	// End is the byte to stop extracting at (inclusive).
	End *uint64
}

type ReadPieceConfig struct {
	ByteRange Range
}

func (c *ReadPieceConfig) ProcessOptions(opts []ReadPieceOption) {
	for _, opt := range opts {
		opt(c)
	}
}

type ReadPieceOption func(c *ReadPieceConfig)

func WithRange(start uint64, end *uint64) ReadPieceOption {
	return func(c *ReadPieceConfig) {
		c.ByteRange = Range{start, end}
	}
}

type PieceAPI interface {
	PieceReaderAPI
	PieceWriterAPI
	PieceResolverAPI

	WritePieceURL(piece uuid.UUID) (url.URL, error)
	ReadPieceURL(piece cid.Cid) (url.URL, error)
}

type PieceWriterAPI interface {
	AllocatePiece(ctx context.Context, allocation PieceAllocation) (*AllocatedPiece, error)
	UploadPiece(ctx context.Context, upload PieceUpload) error
}

type PieceResolverAPI interface {
	ResolvePiece(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error)
	CalculateCommP(ctx context.Context, piece multihash.Multihash) (cid.Cid, error)
}

type PieceReaderAPI interface {
	ReadPiece(ctx context.Context, piece multihash.Multihash, options ...ReadPieceOption) (*PieceReader, error)
}

type ProviderAPI interface {
	RegisterProvider(ctx context.Context, params RegisterProviderParams) (RegisterProviderResults, error)
	GetProviderStatus(ctx context.Context) (GetProviderStatusResults, error)
}
