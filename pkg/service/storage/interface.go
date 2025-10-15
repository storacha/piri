package storage

import (
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/validator"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/service/replicator"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

type Service interface {
	// ID is the storage service identity, used to sign UCAN invocations and receipts.
	ID() principal.Signer
	// PDP handles PDP aggregation
	PDP() pdp.PDP
	// Blobs provides access to the blobs service.
	Blobs() blobs.Blobs
	// Claims provides access to the claims service.
	Claims() claims.Claims
	// Receipts provides access to receipts
	Receipts() receiptstore.ReceiptStore
	// Replicator provides access to the replication service
	Replicator() replicator.Replicator
	// UploadConnection provides the connection details to an upload service
	UploadConnection() client.Connection
	// ClaimValidationContext provides the context required for validating UCANs.
	ClaimValidationContext() validator.ClaimContext
}
