package httpapi

// CreateProofSet types
type (
	CreateProofSetRequest struct {
		RecordKeeper string `json:"recordKeeper"`
		ExtraData    string `json:"extraData"`
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

// AddRoots types
type (
	AddRootsRequest struct {
		Roots     []Root `json:"roots"`
		ExtraData string `json:"extraData,omitempty"`
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
