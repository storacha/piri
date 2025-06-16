package types

import (
	"context"
	"net/url"

	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

type AllocateService interface {
	PDP() pdp.PDP
	Blobs() Blobs
}

// Blobs defines the interface for blob operations
type Blobs interface {
	// Store returns the underlying blob store
	Store() blobstore.Blobstore
	// Allocations returns the allocation store
	Allocations() allocationstore.AllocationStore
	// Presigner returns the request presigner
	Presigner() presigner.RequestPresigner
	// Access returns the access controller
	Access() access.Access
}

// Claims defines the interface for claim operations  
type Claims interface {
	// Store returns the underlying claim store
	Store() claimstore.ClaimStore
	// Publisher returns the publisher
	Publisher() Publisher
}

type Publisher interface {
	// Store is the storage interface for published advertisements.
	Store() store.PublisherStore
	// Publish advertises content claims/commitments found on this node to the
	// storacha network.
	Publish(context.Context, delegation.Delegation) error
}

// TransferRequest represents a blob replication request
type TransferRequest struct {
	// Space is the space to associate with blob.
	Space did.DID
	// Blob is the blob in question.
	Blob types.Blob
	// Source is the location to replicate the blob from.
	Source url.URL
	// Sink is the location to replicate the blob to.
	Sink *url.URL
	// Cause is the invocation responsible for spawning this replication
	// should be a replica/transfer invocation.
	Cause invocation.Invocation
}

// Replicator defines the interface for replication operations
type Replicator interface {
	// Replicate queues a replication task
	Replicate(ctx context.Context, request *TransferRequest) error
	// Start starts the replication service
	Start(ctx context.Context) error
	// Stop stops the replication service
	Stop(ctx context.Context) error
}

// Service provides the storage service interface matching pkg/service/storage
type Service interface {
	ID() principal.Signer
	Blobs() Blobs
	Claims() Claims
	PDP() pdp.PDP
	Receipts() receiptstore.ReceiptStore
	Replicator() Replicator
	UploadConnection() client.Connection
}
