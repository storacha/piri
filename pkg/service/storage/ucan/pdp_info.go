package ucan

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/piece/piece"
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

	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	pdpservice "github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("storage/ucan")

type PDPInfoService interface {
	ID() principal.Signer
	Receipts() receiptstore.ReceiptStore
	PDP() pdpservice.PDP
}

func PDPInfo(storageService PDPInfoService) server.Option {
	return server.WithServiceMethod(
		pdp.InfoAbility,
		server.Provide(
			pdp.Info,
			func(ctx context.Context, cap ucan.Capability[pdp.InfoCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (result.Result[pdp.InfoOk, failure.IPLDBuilderFailure], fx.Effects, error) {
				// if there is a PDP piece either aggregated or queued, we should be able to look up the piece CID
				commpResp, err := storageService.PDP().API().CalculateCommP(ctx, cap.Nb().Blob)
				if err != nil {
					log.Errorf("unable to lookup piece CID: %w", err)
					return nil, nil, err
				}

				// ok, we have a piece CID, let's see if we have an accepted PDP for it

				// generate the invocation that would submit when this was first submitted
				pieceAccept, err := pdp.Accept.Invoke(
					storageService.ID(),
					storageService.ID(),
					storageService.ID().DID().GoString(),
					pdp.AcceptCaveats{
						Blob: cap.Nb().Blob,
					}, delegation.WithNoExpiration())
				if err != nil {
					if store.IsNotFound(err) {
						// no accept found, so this piece is still pending aggregation
						pieceLink, err := piece.FromLink(cidlink.Link{Cid: commpResp.PieceCID})
						if err != nil {
							log.Errorf("creating piece link: %w", err)
							return nil, nil, err
						}
						return result.Ok[pdp.InfoOk, failure.IPLDBuilderFailure](
							pdp.InfoOk{
								Piece:      pieceLink,
								Aggregates: []pdp.InfoAcceptedAggregate{},
							},
						), nil, nil
					}
					log.Errorf("creating pdp accept: %w", err)
					return nil, nil, err
				}
				// look up the receipt for the accept invocation
				rcpt, err := storageService.Receipts().GetByRan(ctx, pieceAccept.Link())
				if err != nil {
					log.Errorf("looking up receipt: %w", err)
					return nil, nil, err
				}
				// rebind the receipt to get the specific types for pdp/accept
				pieceAcceptReceipt, err := receipt.Rebind[pdp.AcceptOk, fdm.FailureModel](rcpt, pdp.AcceptOkType(), fdm.FailureType(), types.Converters...)
				if err != nil {
					log.Errorf("reading piece accept receipt: %w", err)
					return nil, nil, err
				}
				// use the result from the accept receipt to generate the receipt for pdp/info
				return result.MatchResultR3(pieceAcceptReceipt.Out(),
					func(ok pdp.AcceptOk) (result.Result[pdp.InfoOk, failure.IPLDBuilderFailure], fx.Effects, error) {
						return result.Ok[pdp.InfoOk, failure.IPLDBuilderFailure](
							pdp.InfoOk{
								Piece: ok.Piece,
								Aggregates: []pdp.InfoAcceptedAggregate{
									{
										Aggregate:      ok.Aggregate,
										InclusionProof: ok.InclusionProof,
									},
								},
							},
						), nil, nil
					},
					func(err fdm.FailureModel) (result.Result[pdp.InfoOk, failure.IPLDBuilderFailure], fx.Effects, error) {
						return nil, nil, failure.FromFailureModel(err)
					},
				)
			},
		),
	)
}
