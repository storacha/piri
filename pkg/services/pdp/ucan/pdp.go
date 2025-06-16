package ucan

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	fx2 "github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/go-ucanto/validator"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("ucan/method/pdp")

// Info handles replica/allocate capability
type Info struct {
	id            principal.Signer
	receiptsStore receiptstore.ReceiptStore
}

// InfoParams defines dependencies for the handler
type InfoParams struct {
	fx.In
	ID           principal.Signer
	ReceiptStore receiptstore.ReceiptStore
}

// NewInfo creates a new allocate handler
func NewInfo(params InfoParams) *Info {
	return &Info{
		id:            params.ID,
		receiptsStore: params.ReceiptStore,
	}
}

// Option returns the server option for this handler
func (h *Info) Option() server.Option {
	return server.WithServiceMethod(
		pdp.InfoAbility,
		server.Provide(h.Provide()),
	)
}

// Holy generics batman!

// Provide returns the capability parser and handler function
func (h *Info) Provide() (
	validator.CapabilityParser[pdp.InfoCaveats],
	server.HandlerFunc[pdp.InfoCaveats, pdp.InfoOk],
) {
	handler := func(
		c ucan.Capability[pdp.InfoCaveats],
		i invocation.Invocation,
		ictx server.InvocationContext,
	) (pdp.InfoOk, fx2.Effects, error) {
		ctx := context.TODO()
		// generate the invocation that would submit when this was first submitted
		pieceAccept, err := pdp.Accept.Invoke(
			h.id,
			h.id,
			h.id.DID().GoString(),
			pdp.AcceptCaveats{
				Piece: c.Nb().Piece,
			}, delegation.WithNoExpiration())
		if err != nil {
			log.Errorf("creating location commitment: %w", err)
			return pdp.InfoOk{}, nil, failure.FromError(err)
		}
		// look up the receipt for the accept invocation
		rcpt, err := h.receiptsStore.GetByRan(ctx, pieceAccept.Link())
		if err != nil {
			log.Errorf("looking up receipt: %w", err)
			return pdp.InfoOk{}, nil, failure.FromError(err)
		}
		// rebind the receipt to get the specific types for pdp/accept
		pieceAcceptReceipt, err := receipt.Rebind[pdp.AcceptOk, fdm.FailureModel](rcpt, pdp.AcceptOkType(), fdm.FailureType(), types.Converters...)
		if err != nil {
			log.Errorf("reading piece accept receipt: %w", err)
			return pdp.InfoOk{}, nil, failure.FromError(err)
		}
		// use the result from the accept receipt to generate the receipt for pdp/info
		return result.MatchResultR3(pieceAcceptReceipt.Out(),
			func(ok pdp.AcceptOk) (pdp.InfoOk, fx2.Effects, error) {
				return pdp.InfoOk{
					Piece: c.Nb().Piece,
					Aggregates: []pdp.InfoAcceptedAggregate{
						{
							Aggregate:      ok.Aggregate,
							InclusionProof: ok.InclusionProof,
						},
					},
				}, nil, nil
			},
			func(err fdm.FailureModel) (pdp.InfoOk, fx2.Effects, error) {
				return pdp.InfoOk{}, nil, failure.FromFailureModel(err)
			},
		)
	}

	return pdp.Info, handler
}
