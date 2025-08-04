package ucan

import (
	"context"
	"fmt"
	"net/url"

	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/replicator"
	blobhandler "github.com/storacha/piri/pkg/service/storage/handlers/blob"
	replicahandler "github.com/storacha/piri/pkg/service/storage/handlers/replica"
)

type ReplicaAllocateService interface {
	ID() principal.Signer
	PDP() pdp.PDP
	Blobs() blobs.Blobs
	Replicator() replicator.Replicator
}

func ReplicaAllocate(storageService ReplicaAllocateService) server.Option {
	return server.WithServiceMethod(
		replica.AllocateAbility,
		server.Provide(
			replica.Allocate,
			func(ctx context.Context, cap ucan.Capability[replica.AllocateCaveats], inv invocation.Invocation, iCtx server.InvocationContext) (replica.AllocateOk, fx.Effects, error) {
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
				// already allocated space and receieved the data. Therefore, no replication is required.
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
	)
}
