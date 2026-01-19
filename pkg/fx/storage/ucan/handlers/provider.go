package handlers

import (
	"context"
	"slices"

	"go.uber.org/fx"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/piri/pkg/service/storage/ucan"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("retrieval/ucan")

var Module = fx.Module("storage/ucan/handlers",
	fx.Provide(
		fx.Annotate(
			ucan.WithAccessGrantMethod,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ucan.WithBlobAllocateMethod,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ucan.WithBlobAcceptMethod,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ucan.WithPDPInfoMethod,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ucan.WithReplicaAllocateMethod,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			withReceiptLogger,
			fx.ResultTags(`group:"ucan_options"`),
		),
	),
)

var receiptLogAllowList = []string{
	blob.AllocateAbility,
	blob.AcceptAbility,
	replica.AllocateAbility,
}

// withReceiptLogger stores important receipts that we may need to access in the
// future.
func withReceiptLogger(store receiptstore.ReceiptStore) server.Option {
	return server.WithReceiptLogger(func(ctx context.Context, rcpt receipt.AnyReceipt, inv invocation.Invocation) error {
		if len(inv.Capabilities()) != 1 {
			log.Warn("Expected exactly one capability in invocation")
			return nil
		}
		capability := inv.Capabilities()[0]
		if !slices.Contains(receiptLogAllowList, capability.Can()) {
			log.Info("Receipt is for a %s invocation, ignoring", capability.Can())
			return nil
		}
		// Make sure the receipt is self-contained, i.e. it also has invocation blocks
		fullRcpt, err := rcpt.Clone()
		if err != nil {
			return err
		}
		if err := fullRcpt.AttachInvocation(inv); err != nil {
			return err
		}
		if err := store.Put(ctx, fullRcpt); err != nil {
			log.Errorw("putting receipt to store", "error", err)
			return err
		}
		return nil
	})
}
