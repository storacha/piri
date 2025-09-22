package egressbatchstore

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/receipt"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
)

// EgressBatchStore stores batches of receipts. When the batch reaches a certain size,
// the store calculates the CID of the batch, rotates it and creates a new batch to append receipts to.
type EgressBatchStore interface {
	// Append appends a space/content/retrieval receipt to the current batch.
	// If the batch reaches the size limit, it will be rotated. When that happens, Append returns
	// true and the CID of the rotated batch.
	Append(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) (batchRotated bool, rotatedBatchCID cid.Cid, err error)

	// GetBatch returns a batch of receipts by its CID.
	GetBatch(ctx context.Context, cid cid.Cid) (reader io.ReadCloser, err error)
}
