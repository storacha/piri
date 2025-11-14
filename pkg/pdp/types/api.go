package types

import (
	"context"
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
	Hash multihash.Multihash

	// Size of the piece in bytes
	Size int64
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
	Piece     multihash.Multihash
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

type ParkPieceRequest struct {
	Blob       multihash.Multihash
	PieceCID   cid.Cid
	RawSize    int64
	PaddedSize int64
}

type CalculateCommPResponse struct {
	PieceCID   cid.Cid
	RawSize    int64
	PaddedSize int64
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
	PieceResolverAPI
	PieceWriterAPI
	PieceCommPAPI

	// ParkPiece persists a record of a commp cid to the database
	ParkPiece(ctx context.Context, params ParkPieceRequest) error

	// WritePieceURL returns the URL an allocated blob may be uploaded to.
	WritePieceURL(blob uuid.UUID) (url.URL, error)
	// ReadPieceURL returns the URL a blob may be retrieved from.
	ReadPieceURL(blob cid.Cid) (url.URL, error)
}

type PieceWriterAPI interface {
	AllocatePiece(ctx context.Context, allocation PieceAllocation) (*AllocatedPiece, error)
	UploadPiece(ctx context.Context, upload PieceUpload) error
}

type PieceResolverAPI interface {
	// Resolve accepts any multihash and attempts to resolve it to its corresponding hash.
	// For example, if the provided hash is a commp multihash the blob hash will be returned.
	// if the provided hash is not a commp multihash the commp hash will be returned.
	// false if returned if data doesn't exist.
	Resolve(ctx context.Context, data multihash.Multihash) (multihash.Multihash, bool, error)
	// ResolveToPiece accepts a non-commp multihash and returns the commp multihash it corresponds to.
	// If the multihash doesn't exist false is returned without an error.
	ResolveToPiece(ctx context.Context, blob multihash.Multihash) (multihash.Multihash, bool, error)
	// ResolveToBlob accepts a commp multihash and returns the blob multihash it corresponds to.
	// If the commp multihash doesn't exist false is returned without an error.
	ResolveToBlob(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error)
}

type PieceReaderAPI interface {
	// Read returns a `PieceReader` for the provided `data` multihash. An error is returned if the value doesn't exist,
	// of if reading from the store fails. `ReadPieceOption`s may be provided for range queries and resolution before read.
	// Read expects `data` to be the multihash the data was uploaded with. If callers provide a commP multihash to Read
	// they must also provide a resolver method for the operation to succeed, this will almost always be the ResoleToBlob
	// resolver.
	Read(ctx context.Context, data multihash.Multihash, options ...ReadPieceOption) (*PieceReader, error)
	// Has returns true if the provided `data` multihash is present in the store, false otherwise
	Has(ctx context.Context, blob multihash.Multihash) (bool, error)
}

type PieceCommPAPI interface {
	// CalculateCommP accepts a blob multihash and returns a result containing its commp CID, raw and padded size.
	CalculateCommP(ctx context.Context, blob multihash.Multihash) (CalculateCommPResponse, error)
}

type ProviderAPI interface {
	RegisterProvider(ctx context.Context, params RegisterProviderParams) (RegisterProviderResults, error)
	GetProviderStatus(ctx context.Context) (GetProviderStatusResults, error)
}
