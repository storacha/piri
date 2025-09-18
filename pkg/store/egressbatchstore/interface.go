package egressbatchstore

import (
	"context"

	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/receipt"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
)

// EgressBatchStore stores batches of receipts. When the batch reaches a certain size,
// it will be sent to the egress tracking service via an `space/egress/track` invocation.
// It implements http.FileSystem to serve the receipt batches over HTTP.
type EgressBatchStore interface {
	// Append appends a space/content/retrieval receipt to the current batch.
	// If the batch reaches the size limit, it will be sent to the egress tracking service.
	Append(ctx context.Context, rcpt receipt.Receipt[content.RetrieveOk, fdm.FailureModel]) error

	// Flush forces the current batch to be finalized and sent to the egress tracking service.
	// This is useful for periodic flushes.
	Flush(ctx context.Context) error
}
