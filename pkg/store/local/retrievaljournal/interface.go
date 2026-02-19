package retrievaljournal

import (
	"context"
	"io"
	"iter"

	"github.com/ipfs/go-cid"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/failure"
	"github.com/storacha/go-ucanto/core/receipt"
)

// Journal stores batches of receipts. When the batch reaches a certain size,
// the store calculates the CID of the batch, rotates it and creates a new batch to append receipts to.
type Journal interface {
	// Append appends a space/content/retrieval receipt to the current batch.
	// If the batch reaches the size limit, it will be rotated. When that happens, Append returns
	// true and the CID of the rotated batch.
	Append(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, failure.FailureModel]) (batchRotated bool, rotatedBatchCID cid.Cid, err error)

	// GetBatch returns a batch of receipts by its CID.
	GetBatch(ctx context.Context, cid cid.Cid) (reader io.ReadCloser, err error)

	// List returns the CIDs of all rotated batches.
	List(ctx context.Context) (iter.Seq[cid.Cid], error)

	// Remove removes a batch by its CID.
	Remove(ctx context.Context, cid cid.Cid) error
}

// ForceRotator is an optional interface that a Journal can implement to allow
// external forces to trigger batch rotation.
type ForceRotator interface {
	// ForceRotate causes a rotation of the current journal batch, regardless of
	// its size or any other factors that may influence rotation normally. It
	// returns a CID that identifies the rotated batch.
	//
	// Note: If there are no entries in the current batch then rotation is not
	// possible and it returns (false, [cid.Undef], nil).
	ForceRotate(ctx context.Context) (bool, cid.Cid, error)
}
