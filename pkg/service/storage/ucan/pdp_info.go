package ucan

import (
	"context"
	"fmt"

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
	piece2 "github.com/storacha/piri/pkg/pdp/piece"

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
				if storageService.PDP() == nil {
					log.Error("PDPInfo requested but PDP service is not available")
					return nil, nil, failure.FromError(fmt.Errorf("PDP service not avaliable"))
				}

				// try and resolve the blob to its derived pieceCID (commp)
				resolvedCommp, found, err := storageService.PDP().API().ResolveToPiece(ctx, cap.Nb().Blob)
				if err != nil {
					log.Errorw("failed to resolve PDP api", "error", err)
					return nil, nil, failure.FromError(fmt.Errorf("failed to resolve PDP API: %w", err))
				}
				if !found {
					// we didn't find the commp for this blob, compute it on demand, this means it hasn't been computed yet
					// and is still likely in the pipeline.
					// TODO(forrest): this is a bit wastefully, we could instead poll for it to be resolved yolo-ing for now
					commpResp, err := storageService.PDP().API().CalculateCommP(ctx, cap.Nb().Blob)
					if err != nil {
						log.Errorw("failed to compute commp for digest", "digest", cap.Nb().Blob.String(), "error", err)
						return nil, nil, failure.FromError(fmt.Errorf("failed to compute commp for digest: %w", err))
					}
					pieceLink, err := piece.FromLink(cidlink.Link{Cid: commpResp.PieceCID})
					if err != nil {
						log.Errorw("failed to create piece link for commp piece", "piece", commpResp.PieceCID, "error", err)
						return nil, nil, failure.FromError(fmt.Errorf("failed to create piece link for commp piece: %w", err))
					}

					// since we could resolve it, this means the blob has not been aggregated yet
					// so this blob is still pending aggregation
					return result.Ok[pdp.InfoOk, failure.IPLDBuilderFailure](
						pdp.InfoOk{
							Piece:      pieceLink,
							Aggregates: []pdp.InfoAcceptedAggregate{},
						},
					), nil, nil

				}
				// else we resolved the blob to a piece, so a commp has been computed for it, though it still may not
				// have been aggregated. For example if this is a small blob, then it may take time for aggregation to occure.

				// generate the invocation that would submit when this was first submitted, allowing the
				// receipt to be retrieved for it from the receipt store.
				pieceAccept, err := pdp.Accept.Invoke(
					storageService.ID(),
					storageService.ID(),
					storageService.ID().DID().GoString(),
					pdp.AcceptCaveats{
						Blob: cap.Nb().Blob,
					}, delegation.WithNoExpiration())
				if err != nil {
					log.Errorw("unable to invoke pdp accept", "error", err)
					return nil, nil, failure.FromError(fmt.Errorf("unable to invoke pdp accept: %w", err))
				}

				// look up the receipt for the accept invocation
				rcpt, err := storageService.Receipts().GetByRan(ctx, pieceAccept.Link())
				if err != nil {
					// This can happen when a piece is still awaiting aggregation
					// TODO here is where a polling mechanism could be helpful
					log.Errorw("looking up receipt", "error", err)
					return nil, nil, failure.FromError(fmt.Errorf("looking up receipt: %w", err))
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
						// sanity check
						commpCID := piece2.MultihashToCommpCID(resolvedCommp)
						if ok.Piece.Link().String() != commpCID.String() {
							log.Errorw("resolved piece CID does not match receipt piece CID", "expect", commpCID, "got", ok.Piece.Link().String())
							return nil, nil, failure.FromError(fmt.Errorf("resolved piece CID %s does not match receipt piece CID %s", ok.Piece.Link().String(), commpCID.String()))
						}
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
