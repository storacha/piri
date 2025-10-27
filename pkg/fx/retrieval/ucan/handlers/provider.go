package handlers

import (
	"context"

	"go.uber.org/fx"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	ucanserver "github.com/storacha/go-ucanto/server"
	ucanretrieval "github.com/storacha/go-ucanto/server/retrieval"

	"github.com/storacha/piri/pkg/service/egresstracker"
	"github.com/storacha/piri/pkg/service/retrieval/ucan"
)

var log = logging.Logger("retrieval/ucan")

var Module = fx.Module("retrieval/ucan/handlers",
	fx.Provide(
		fx.Annotate(
			ucan.BlobRetrieve,
			fx.ResultTags(`group:"ucan_retrieval_options"`),
		),
		fx.Annotate(
			ucan.SpaceContentRetrieve,
			fx.ResultTags(`group:"ucan_retrieval_options"`),
		),
		fx.Annotate(
			withErrorHandler,
			fx.ResultTags(`group:"ucan_retrieval_options"`),
		),
		fx.Annotate(
			withReceiptLogger,
			fx.ResultTags(`group:"ucan_retrieval_options"`),
		),
	),
)

func withErrorHandler() ucanretrieval.Option {
	return ucanretrieval.WithErrorHandler(func(err ucanserver.HandlerExecutionError[any]) {
		l := log.With("error", err.Error())
		if s := err.Stack(); s != "" {
			l = l.With("stack", s)
		}
		l.Error("ucan retrieval handler execution error")
	})
}

func withReceiptLogger(ets *egresstracker.Service) ucanretrieval.Option {
	return ucanretrieval.WithReceiptLogger(func(_ context.Context, rcpt receipt.AnyReceipt, inv invocation.Invocation) error {
		// Egress tracking is optional, the service will be nil if it is disabled
		if ets == nil {
			log.Warn("Egress tracking is not configured")
			return nil
		}

		if len(inv.Capabilities()) != 1 {
			log.Warn("Expected exactly one capability in invocation")
			return nil
		}

		capability := inv.Capabilities()[0]
		if capability.Can() != content.RetrieveAbility {
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

		retrievalRcpt, err := receipt.Rebind[content.RetrieveOk, fdm.FailureModel](fullRcpt, content.RetrieveOkType(), fdm.FailureType())
		if err != nil {
			return err
		}

		if err := ets.AddReceipt(context.Background(), retrievalRcpt); err != nil {
			return err
		}

		return nil
	})
}
