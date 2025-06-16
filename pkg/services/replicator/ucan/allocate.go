package ucan

import (
	"context"
	"fmt"
	"net/url"

	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	fx2 "github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/go-ucanto/validator"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/pdp"
	blobhandler "github.com/storacha/piri/pkg/services/blob/ucan"
	"github.com/storacha/piri/pkg/services/errors"
	"github.com/storacha/piri/pkg/services/types"
)

// Allocate handles replica/allocate capability
type Allocate struct {
	id                 principal.Signer
	blobService        types.Blobs
	pdpService         pdp.PDP
	replicationService types.Replicator
}

func (h *Allocate) PDP() pdp.PDP {
	return h.pdpService
}

func (h *Allocate) Blobs() types.Blobs {
	return h.blobService
}

// AllocateParams defines dependencies for the handler
type AllocateParams struct {
	fx.In
	ID                 principal.Signer
	BlobService        types.Blobs
	PDPService         pdp.PDP `optional:"true"`
	ReplicationService types.Replicator
}

// NewAllocate creates a new allocate handler
func NewAllocate(params AllocateParams) *Allocate {
	return &Allocate{
		id:                 params.ID,
		blobService:        params.BlobService,
		pdpService:         params.PDPService,
		replicationService: params.ReplicationService,
	}
}

// Option returns the server option for this handler
func (h *Allocate) Option() server.Option {
	return server.WithServiceMethod(
		replica.AllocateAbility,
		server.Provide(h.Provide()),
	)
}

// Holy generics batman!

// Provide returns the capability parser and handler function
func (h *Allocate) Provide() (
	validator.CapabilityParser[replica.AllocateCaveats],
	server.HandlerFunc[replica.AllocateCaveats, replica.AllocateOk],
) {
	handler := func(
		c ucan.Capability[replica.AllocateCaveats],
		i invocation.Invocation,
		ictx server.InvocationContext,
	) (replica.AllocateOk, fx2.Effects, error) {
		//
		// UCAN Validation
		//

		// only service principal can perform an allocation
		if c.With() != ictx.ID().DID().String() {
			return replica.AllocateOk{}, nil, errors.NewUnsupportedCapabilityError(c)
		}

		//
		// end UCAN Validation
		//

		// read the location claim from this invocation to obtain the DID of the URL
		// to replicate from on the primary storage node.
		br, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(i.Blocks()))
		if err != nil {
			return replica.AllocateOk{}, nil, failure.FromError(err)
		}
		claim, err := delegation.NewDelegationView(c.Nb().Site, br)
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
		resp, err := blobhandler.Allocate(ctx, h, &blobhandler.AllocateRequest{
			Space: c.Nb().Space,
			Blob:  c.Nb().Blob,
			Cause: i.Link(),
		})
		if err != nil {
			return replica.AllocateOk{}, nil, failure.FromError(err)
		}

		// create the transfer invocation: an fx of the allocate invocation receipt.
		trnsfInv, err := replica.Transfer.Invoke(
			h.id,
			h.id,
			h.id.DID().String(),
			replica.TransferCaveats{
				Space: c.Nb().Space,
				Blob: captypes.Blob{
					Digest: c.Nb().Blob.Digest,
					// use the allocation response size since it may be zero, indicating
					// an allocation already exists, and may or may not require transfer
					Size: resp.Size,
				},
				Site:  c.Nb().Site,
				Cause: i.Link(),
			},
		)
		if err != nil {
			return replica.AllocateOk{}, nil, failure.FromError(err)
		}
		for block, err := range i.Blocks() {
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
		if err := h.replicationService.Replicate(ctx, &types.TransferRequest{
			Space:  c.Nb().Space,
			Blob:   c.Nb().Blob,
			Source: replicaAddress,
			Sink:   sink,
			Cause:  trnsfInv,
		}); err != nil {
			return replica.AllocateOk{}, nil, failure.FromError(fmt.Errorf("failed to enqueue replication task: %w", err))
		}

		// Create a Promise for the transfer invocation
		transferPromise := captypes.Promise{
			UcanAwait: captypes.Await{
				Selector: ".out.ok",
				Link:     trnsfInv.Link(),
			},
		}

		return replica.AllocateOk{
			Size: resp.Size,
			Site: transferPromise,
		}, fx2.NewEffects(fx2.WithFork(fx2.FromInvocation(trnsfInv))), nil
	}

	return replica.Allocate, handler
}
