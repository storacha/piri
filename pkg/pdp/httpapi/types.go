package httpapi

import (
	"github.com/ethereum/go-ethereum/common"
)

// CreateProofSet types
type (
	CreateProofSetRequest struct {
		RecordKeeper string `json:"recordKeeper"`
	}

	CreateProofSetResponse struct {
		TxHash   string `json:"txHash"`
		Location string `json:"location"`
	}
)

// GetProofSetStatus types
type (
	// NB: there is no request type for status, the transaction is a url parameter

	ProofSetStatusResponse struct {
		CreateMessageHash string  `json:"createMessageHash"`
		ProofsetCreated   bool    `json:"proofsetCreated"`
		Service           string  `json:"service"`
		TxStatus          string  `json:"txStatus"`
		OK                *bool   `json:"ok"`
		ProofSetId        *uint64 `json:"proofSetId,omitempty"`
	}
)

// GetProofSet types
type (
	GetProofSetResponse struct {
		ID                 uint64      `json:"id"`
		NextChallengeEpoch *int64      `json:"nextChallengeEpoch"`
		Roots              []RootEntry `json:"roots"`

		// piri only - optinal
		PreviousChallengeEpoch *int64 `json:"previousChallengeEpoch,omitempty"`
		ProvingPeriod          *int64 `json:"provingPeriod,omitempty"`
		ChallengeWindow        *int64 `json:"challengeWindow,omitempty"`
	}

	RootEntry struct {
		RootID        uint64 `json:"rootId"`
		RootCID       string `json:"rootCid"`
		SubrootCID    string `json:"subrootCid"`
		SubrootOffset int64  `json:"subrootOffset"`
	}
)

type (
	ListProofSetsResponse []ProofSetEntry
	ProofSetEntry         struct {
		ID                     uint64      `json:"id"`
		Initialized            bool        `json:"initialized"`
		Roots                  []RootEntry `json:"roots"`
		NextChallengeEpoch     *int64      `json:"nextChallengeEpoch,omitempty"`
		PreviousChallengeEpoch *int64      `json:"previousChallengeEpoch,omitempty"`
		ProvingPeriod          *int64      `json:"provingPeriod,omitempty"`
		ChallengeWindow        *int64      `json:"challengeWindow,omitempty"`
	}
)

type (
	GetProofSetStateResponse struct {
		ID uint64 `json:"id"`
		// if the proof set has been initialized with a root, and is expecting proofs to be submitted.
		Initialized bool `json:"initialized"`
		// When the next challenge for a proof will be issued
		NextChallengeEpoch int64 `json:"nextChallengeEpoch"`
		// When the last challenge for a proof was issued
		PreviousChallengeEpoch int64 `json:"previousChallengeEpoch"`
		// The proving period of this proof set
		ProvingPeriod int64 `json:"provingPeriod"`
		// The challenge window of this proof set
		ChallengeWindow int64 `json:"challengeWindow"`
		// The current epoch of the chain
		CurrentEpoch int64 `json:"currentEpoch"`
		// true if a challenge has been issued: CurrentEpoch >= NextChallengeEpoch
		ChallengedIssued bool `json:"challengedIssued"`
		// true if in challenge window: CurrentEpoch < NextChallengeEpoch + ChallengeWindow
		InChallengeWindow bool `json:"inChallengeWindow"`
		// true if we missed the challenge: CurrentEpoch > NextChallengeEpoch + ChallengeWindow
		IsInFaultState bool `json:"isInFaultState"`
		// true if we submitted a proof for the current ChallengeWindow
		HasProven bool `json:"hasProven"`
		// true if the node is currently generating a proof
		IsProving bool `json:"isProving"`
		// the state of the proof set present in the contract
		ContractState ProofSetContractState `json:"contractState"`
	}

	ProofSetContractState struct {
		// owners of the proof set
		Owners []common.Address `json:"owners"`
		// The start of the NEXT OPEN proving period's challenge window
		NextChallengeWindowStart uint64 `json:"nextChallengeWindowStart"`
		// the epoch of the next challenge
		NextChallengeEpoch uint64 `json:"nextChallengeEpoch"`
		// Max number of epochs between two consecutive proofs
		MaxProvingPeriod uint64 `json:"maxProvingPeriod"`
		// challengeWindow Number of epochs for the challenge window
		ChallengeWindow uint64 `json:"challengeWindow"`
		//index of the most recently added leaf that is challengeable in the current proving period
		ChallengeRange uint64 `json:"challengeRange"`
		// piece ids of the pieces scheduled for removal at the start of the next proving period
		ScheduledRemovals []uint64 `json:"scheduledRemovals"`
		// estimated cost of submitting a proof
		ProofFee uint64 `json:"proofFee"`
		// estimated cost of submitting a proof with buffer applied
		ProofFeeBuffered uint64 `json:"proofFeeBuffered"`
	}
)

// AddRoots types
type (
	AddRootsRequest struct {
		Roots []Root `json:"roots"`
	}
	Root struct {
		RootCID  string         `json:"rootCid"`
		Subroots []SubrootEntry `json:"subroots"`
	}

	SubrootEntry struct {
		SubrootCID string `json:"subrootCid"`
	}

	AddRootsResponse struct {
		TxHash string `json:"txHash"`
	}
)

type (
	RemoveRootResponse struct {
		TxHash string `json:"txHash"`
	}
)
type (
	AddPieceRequest struct {
		Check  PieceHash `json:"check"`
		Notify string    `json:"notify,omitempty"`
	}

	PieceHash struct {
		// Name of the hash function used
		// sha2-256-trunc254-padded - CommP
		// sha2-256 - Blob sha256
		Name string `json:"name"`

		// hex encoded hash
		Hash string `json:"hash"`

		// Size of the piece in bytes
		Size int64 `json:"size"`
	}

	AddPieceResponse struct {
		Allocated bool   `json:"allocated"`
		PieceCID  string `json:"pieceCid"`
		UploadID  string `json:"uploadId"`
	}
)
type (
	FoundPieceResponse struct {
		PieceCID string `json:"piece_cid"`
	}
)

// RegisterProvider types
type (
	RegisterProviderRequest struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	RegisterProviderResponse struct {
		TxHash      string `json:"txHash,omitempty"`
		Address     string `json:"address,omitempty"`
		Payee       string `json:"payee,omitempty"`
		ID          uint64 `json:"id,omitempty"`
		IsActive    bool   `json:"isActive,omitempty"`
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
	}
)

// GetProviderStatus types
type (
	GetProviderStatusResponse struct {
		ID                 uint64 `json:"id"`
		Address            string `json:"address"`
		Payee              string `json:"payee"`
		IsRegistered       bool   `json:"isRegistered"`
		IsActive           bool   `json:"isActive"`
		Name               string `json:"name"`
		Description        string `json:"description"`
		RegistrationStatus string `json:"registrationStatus"`
		IsApproved         bool   `json:"isApproved"`
	}
)
