package retrievaljournal

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/receipt"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
)

// Journal stores batches of receipts. When the batch reaches a certain size,
// the store calculates the CID of the batch, rotates it and creates a new batch to append receipts to.
type Journal interface {
	// Append appends a space/content/retrieval receipt to the current batch.
	// If the batch reaches the size limit, it will be rotated. When that happens, Append returns
	// true and the CID of the rotated batch.
	Append(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) (batchRotated bool, rotatedBatchCID cid.Cid, err error)

	// GetBatch returns a batch of receipts by its CID.
	GetBatch(ctx context.Context, cid cid.Cid) (reader io.ReadCloser, err error)

	// List returns the CIDs of all rotated batches.
	List(ctx context.Context) ([]cid.Cid, error)

	// Remove removes a batch by its CID.
	Remove(ctx context.Context, cid cid.Cid) error
}
