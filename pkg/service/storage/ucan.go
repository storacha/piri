package storage

import (
	"context"
	"fmt"
	"net/url"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"

	blobhandler "github.com/storacha/piri/pkg/service/storage/handlers/blob"
	replicahandler "github.com/storacha/piri/pkg/service/storage/handlers/replica"
)

var log = logging.Logger("storage")

const maxUploadSize = 127 * (1 << 25)

func NewUCANServer(storageService Service, options ...server.Option) (server.ServerView, error) {
	options = append(
		options,
		server.WithServiceMethod(
			blob.AllocateAbility,
			server.Provide(
				blob.Allocate,
				func(cap ucan.Capability[blob.AllocateCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (blob.AllocateOk, fx.Effects, error) {
					//
					// UCAN Validation
					//

					// only service principal can perform an allocation
					if cap.With() != iCtx.ID().DID().String() {
						return blob.AllocateOk{}, nil, NewUnsupportedCapabilityError(cap)
					}

					// enforce max upload size requirements
					if cap.Nb().Blob.Size > maxUploadSize {
						return blob.AllocateOk{}, nil, NewBlobSizeLimitExceededError(cap.Nb().Blob.Size, maxUploadSize)
					}

					//
					// end UCAN Validation
					//

					// FIXME: use a real context, requires changes to server
					ctx := context.TODO()
					resp, err := blobhandler.Allocate(ctx, storageService, &blobhandler.AllocateRequest{
						Space: cap.Nb().Space,
						Blob:  cap.Nb().Blob,
						Cause: inv.Link(),
					})
					if err != nil {
						return blob.AllocateOk{}, nil, failure.FromError(err)
					}

					return blob.AllocateOk{
						Size:    resp.Size,
						Address: resp.Address,
					}, nil, nil
				},
			),
		),
		server.WithServiceMethod(
			blob.AcceptAbility,
			server.Provide(
				blob.Accept,
				func(cap ucan.Capability[blob.AcceptCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (blob.AcceptOk, fx.Effects, error) {
					//
					// UCAN Validation
					//

					// only service principal can perform an allocation
					if cap.With() != iCtx.ID().DID().String() {
						return blob.AcceptOk{}, nil, NewUnsupportedCapabilityError(cap)
					}

					//
					// end UCAN Validation
					//

					// FIXME: use a real context, requires changes to server
					ctx := context.TODO()
					resp, err := blobhandler.Accept(ctx, storageService, &blobhandler.AcceptRequest{
						Space: cap.Nb().Space,
						Blob:  cap.Nb().Blob,
						Put:   cap.Nb().Put,
					})
					if err != nil {
						return blob.AcceptOk{}, nil, failure.FromError(err)
					}
					forks := []fx.Effect{fx.FromInvocation(resp.Claim)}
					res := blob.AcceptOk{
						Site: resp.Claim.Link(),
					}
					if resp.PDP != nil {
						forks = append(forks, fx.FromInvocation(resp.PDP))
						tmp := resp.PDP.Link()
						res.PDP = &tmp
					}

					return res, fx.NewEffects(fx.WithFork(forks...)), nil
				},
			),
		),
		server.WithServiceMethod(
			pdp.InfoAbility,
			server.Provide(
				pdp.Info,
				func(cap ucan.Capability[pdp.InfoCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (pdp.InfoOk, fx.Effects, error) {
					ctx := context.TODO()
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
		),
		server.WithServiceMethod(
			replica.AllocateAbility,
			server.Provide(
				replica.Allocate,
				func(cap ucan.Capability[replica.AllocateCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (replica.AllocateOk, fx.Effects, error) {
					//
					// UCAN Validation
					//

					// only service principal can perform an allocation
					if cap.With() != iCtx.ID().DID().String() {
						return replica.AllocateOk{}, nil, NewUnsupportedCapabilityError(cap)
					}

					//
					// end UCAN Validation
					//

					// read the location claim from this invocation to obtain the DID of the URL
					// to replicate from on the primary storage node.
					br, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(inv.Blocks()))
					if err != nil {
						return replica.AllocateOk{}, nil, failure.FromError(err)
					}
					claim, err := delegation.NewDelegationView(cap.Nb().Site, br)
					if err != nil {
						return replica.AllocateOk{}, nil, failure.FromError(err)
					}

					// TODO since there is a slice of capabilities here we need to validate the 0th is the correct one
					// unsure what `With()` should be compared with for a capability.
					lc, err := assert.LocationCaveatsReader.Read(claim.Capabilities()[0].Nb())
					if err != nil {
						return replica.AllocateOk{}, nil, failure.FromError(err)
					}

					if len(lc.Location) < 1 {
						return replica.AllocateOk{}, nil, failure.FromError(fmt.Errorf("location missing from location claim"))
					}

					// TODO: which one do we pick if > 1?
					replicaAddress := lc.Location[0]

					// FIXME: use a real context, requires changes to server
					ctx := context.TODO()
					resp, err := blobhandler.Allocate(ctx, storageService, &blobhandler.AllocateRequest{
						Space: cap.Nb().Space,
						Blob:  cap.Nb().Blob,
						Cause: inv.Link(),
					})
					if err != nil {
						return replica.AllocateOk{}, nil, failure.FromError(err)
					}

					// create the transfer invocation: an fx of the allocate invocation receipt.
					trnsfInv, err := replica.Transfer.Invoke(
						storageService.ID(),
						storageService.ID(),
						storageService.ID().DID().GoString(),
						replica.TransferCaveats{
							Space: cap.Nb().Space,
							Blob: types.Blob{
								Digest: cap.Nb().Blob.Digest,
								// use the allocation response size since it may be zero, indicating
								// an allocation already exists, and may or may not require transfer
								Size: resp.Size,
							},
							Site:  cap.Nb().Site,
							Cause: inv.Link(),
						},
					)
					if err != nil {
						return replica.AllocateOk{}, nil, failure.FromError(err)
					}
					for block, err := range inv.Blocks() {
						if err != nil {
							return replica.AllocateOk{}, nil, fmt.Errorf("iterating replica allocate invocation blocks: %w", err)
						}
						if err := trnsfInv.Attach(block); err != nil {
							return replica.AllocateOk{}, nil, fmt.Errorf("failed to replica allocate invocation block (%s) to transfer invocation: %w", block.Link().String(), err)
						}
					}
					// iff we didn't allocate space for the data, and didn't provide an address, then it means we have
					// already allocated space and received the data. Therefore, no replication is required.
					sink := new(url.URL)
					if resp.Size == 0 && resp.Address == nil {
						sink = nil
					} else {
						// we need to replicate
						sink = &resp.Address.URL
					}

					// will run replication async, sending the receipt of the transfer invocation
					// to the upload service.
					if err := storageService.Replicator().Replicate(ctx, &replicahandler.TransferRequest{
						Space:  cap.Nb().Space,
						Blob:   cap.Nb().Blob,
						Source: replicaAddress,
						Sink:   sink,
						Cause:  trnsfInv,
					}); err != nil {
						return replica.AllocateOk{}, nil, failure.FromError(fmt.Errorf("failed to enqueue replication task: %w", err))
					}

					// Create a Promise for the transfer invocation
					transferPromise := types.Promise{
						UcanAwait: types.Await{
							Selector: ".out.ok",
							Link:     trnsfInv.Link(),
						},
					}

					return replica.AllocateOk{
						Size: resp.Size,
						Site: transferPromise,
					}, fx.NewEffects(fx.WithFork(fx.FromInvocation(trnsfInv))), nil
				},
			),
		),
	)

	return server.NewServer(storageService.ID(), options...)
}
