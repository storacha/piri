package store

import (
	"context"
	"time"

	"github.com/storacha/go-ucanto/did"
)

// StorageProviderInfo represents the provider metadata table
type StorageProviderInfo struct {
	// Provider is the did:key of the storage node.
	Provider string `json:"provider" db:"provider"`
	// Endpoint is the domain the storage node is reachable at.
	Endpoint string `json:"endpoint" db:"endpoint"`
	// Address is the ethereum address the storage node uses to submit proofs.
	Address string `json:"address" db:"address"`
	// ProofSet is the proof set ID the storage node will use.
	ProofSet uint64 `json:"proof_set" db:"proof_set"`
	// OperatorEmail is the email address of the storage nodes operator.
	OperatorEmail string `json:"operator_email" db:"operator_email"`
	// Proof is a delegation allowing the upload service to send invocations to the storage node.
	Proof string `json:"proof" db:"proof"`
	// InsertedAt is the time this record was created.
	InsertedAt time.Time `json:"inserted_at" db:"inserted_at"`
	// UpdatedAt is the time this record was last modified.
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Store interface {
	IsAllowedDID(ctx context.Context, did did.DID) (bool, error)
	IsRegisteredDID(ctx context.Context, did did.DID) (bool, error)
	RegisterProvider(ctx context.Context, provider StorageProviderInfo) error
}
