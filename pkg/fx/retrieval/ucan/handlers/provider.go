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

	"github.com/storacha/piri/pkg/service/egresstracking"
	"github.com/storacha/piri/pkg/service/retrieval/ucan"
)

var log = logging.Logger("retrieval/ucan")

var Module = fx.Module("retrieval/ucan/handlers",
	fx.Provide(
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

func withReceiptLogger(ets *egresstracking.EgressTrackingService) ucanretrieval.Option {
	return ucanretrieval.WithReceiptLogger(func(_ context.Context, rcpt receipt.AnyReceipt, inv invocation.Invocation) {
		// Egress Tracking is optional, the service will be nil if it is disabled
		if ets == nil {
			return
		}

		// Collect the receipt in a goroutine to avoid blocking the handler
		go func() {
			// Make sure the receipt is self-contained, i.e. it also has invocation blocks
			fullRcpt, err := rcpt.Clone()
			if err != nil {
				log.Errorw("cloning receipt", "error", err)
				return
			}

			if err := fullRcpt.AttachInvocation(inv); err != nil {
				log.Errorw("attaching invocation to receipt", "error", err)
				return
			}

			retrievalRcpt, err := receipt.Rebind[content.RetrieveOk, fdm.FailureModel](fullRcpt, content.RetrieveOkType(), fdm.FailureType())
			if err != nil {
				log.Errorw("rebinding receipt", "error", err)
				return
			}

			if err := ets.AddReceipt(context.Background(), retrievalRcpt); err != nil {
				log.Errorw("adding receipt to egress tracking service", "error", err)
				return
			}
		}()
	})
}
