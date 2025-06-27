package storage

import (
	logging "github.com/ipfs/go-log"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/services/types"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("storage-service")

// service implements the Service interface
type service struct {
	id               principal.Signer
	blobs            types.Blobs
	claims           types.Claims
	pdp              pdp.PDP
	receiptStore     receiptstore.ReceiptStore
	replicator       types.Replicator
	uploadConnection client.Connection
}

// ServiceParams defines the dependencies for the storage service
type ServiceParams struct {
	fx.In

	ID                principal.Signer
	BlobService       types.Blobs
	ClaimService      types.Claims
	PDP               pdp.PDP
	ReceiptStore      receiptstore.ReceiptStore
	ReplicatorService types.Replicator
	UploadConnection  client.Connection `name:"upload"`
}

// NewService creates a new storage service with all dependencies injected
func NewService(params ServiceParams) types.Service {
	log.Infof("Creating storage service with ID: %s", params.ID.DID())

	return &service{
		id:               params.ID,
		blobs:            params.BlobService,
		claims:           params.ClaimService,
		pdp:              params.PDP,
		receiptStore:     params.ReceiptStore,
		replicator:       params.ReplicatorService,
		uploadConnection: params.UploadConnection,
	}
}

// ID returns the service identity
func (s *service) ID() principal.Signer {
	return s.id
}

// Blobs returns the blob service
func (s *service) Blobs() types.Blobs {
	return s.blobs
}

// Claims returns the claim service
func (s *service) Claims() types.Claims {
	return s.claims
}

// PDP returns the PDP service
func (s *service) PDP() pdp.PDP {
	return s.pdp
}

// Receipts returns the receipt store
func (s *service) Receipts() receiptstore.ReceiptStore {
	return s.receiptStore
}

// Replicator returns the replicator service
func (s *service) Replicator() types.Replicator {
	return s.replicator
}

// UploadConnection returns the upload service connection
func (s *service) UploadConnection() client.Connection {
	return s.uploadConnection
}
