package ucan

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("storage/ucan")

type PDPInfoService interface {
	ID() principal.Signer
	Receipts() receiptstore.ReceiptStore
}

func PDPInfo(storageService PDPInfoService) server.Option {
	return server.WithServiceMethod(
		pdp.InfoAbility,
		server.Provide(
			pdp.Info,
			func(ctx context.Context, cap ucan.Capability[pdp.InfoCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (pdp.InfoOk, fx.Effects, error) {
				// generate the invocation that would submit when this was first submitted
				pieceAccept, err := pdp.Accept.Invoke(
					storageService.ID(),
					storageService.ID(),
					storageService.ID().DID().GoString(),
					pdp.AcceptCaveats{
						Piece: cap.Nb().Piece,
					}, delegation.WithNoExpiration())
				if err != nil {
					log.Errorf("creating location commitment: %w", err)
					return pdp.InfoOk{}, nil, failure.FromError(err)
				}
				// look up the receipt for the accept invocation
				rcpt, err := storageService.Receipts().GetByRan(ctx, pieceAccept.Link())
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
					func(ok pdp.AcceptOk) (pdp.InfoOk, fx.Effects, error) {
						return pdp.InfoOk{
							Piece: cap.Nb().Piece,
							Aggregates: []pdp.InfoAcceptedAggregate{
								{
									Aggregate:      ok.Aggregate,
									InclusionProof: ok.InclusionProof,
								},
							},
						}, nil, nil
					},
					func(err fdm.FailureModel) (pdp.InfoOk, fx.Effects, error) {
						return pdp.InfoOk{}, nil, failure.FromFailureModel(err)
					},
				)
			},
		),
	)
}
