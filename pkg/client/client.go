package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	uhttp "github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
)

var ErrNoReceipt = errors.New("no error for invocation")
var ErrIncorrectCapability = errors.New("did not receive expected capability")

type Config struct {
	ID principal.Signer
	// StorageNodeID is the DID of the storage node.
	StorageNodeID ucan.Principal
	// StorageNodeURL is the URL of the storage node UCAN endpoint.
	StorageNodeURL url.URL
	// StorageProof is a delegation allowing the upload service to invoke
	// blob/allocate and blob/accept on the storage node.
	StorageProof delegation.Proof
}

// Client simulates actions taken by the upload service in response to
// client invocations.
type Client struct {
	cfg  Config
	conn client.Connection
}

// BlobAllocate sends a blob/allocate invocation to the storage node and returns the
// upload address if required (i.e. it may be nil if the storage node already
// has the blob).
func (s *Client) BlobAllocate(ctx context.Context, space did.DID, digest multihash.Multihash, size uint64, cause datamodel.Link) (*blob.Address, error) {
	inv, err := blob.Allocate.Invoke(
		s.cfg.ID,
		s.cfg.StorageNodeID,
		s.cfg.StorageNodeID.DID().String(),
		blob.AllocateCaveats{
			Space: space,
			Blob: types.Blob{
				Digest: digest,
				Size:   size,
			},
			Cause: cause,
		},
		delegation.WithProof(s.cfg.StorageProof),
	)
	if err != nil {
		return nil, fmt.Errorf("generating invocation: %w", err)
	}
	res, err := client.Execute(ctx, []invocation.Invocation{inv}, s.conn)
	if err != nil {
		return nil, fmt.Errorf("sending invocation: %w", err)
	}
	reader, err := receipt.NewReceiptReaderFromTypes[blob.AllocateOk, fdm.FailureModel](blob.AllocateOkType(), fdm.FailureType(), types.Converters...)
	if err != nil {
		return nil, fmt.Errorf("generating receipt reader: %w", err)
	}
	rcptLink, ok := res.Get(inv.Link())
	if !ok {
		return nil, ErrNoReceipt
	}
	rcpt, err := reader.Read(rcptLink, res.Blocks())
	if err != nil {
		return nil, fmt.Errorf("reading receipt: %w", err)
	}
	alloc, err := result.Unwrap(result.MapError(rcpt.Out(), failure.FromFailureModel))
	if err != nil {
		return nil, fmt.Errorf("received error from storage node: %w", err)
	}
	return alloc.Address, nil
}

// Blob sends a blob/accept invocation to the storage node and
// returns the location commitment and piece/accept invocation
type BlobAcceptResult struct {
	LocationCommitment assert.LocationCaveats
	PDPAccept          *pdp.AcceptCaveats
}

func (s *Client) BlobAccept(ctx context.Context, space did.DID, digest multihash.Multihash, size uint64, putInv datamodel.Link) (*BlobAcceptResult, error) {

	inv, err := blob.Accept.Invoke(
		s.cfg.ID,
		s.cfg.StorageNodeID,
		s.cfg.StorageNodeID.DID().String(),
		blob.AcceptCaveats{
			Space: space,
			Blob: types.Blob{
				Digest: digest,
				Size:   size,
			},
			Put: blob.Promise{
				UcanAwait: blob.Await{
					Selector: ".out.ok",
					Link:     putInv,
				},
			},
		},
		delegation.WithProof(s.cfg.StorageProof),
	)
	if err != nil {
		return nil, fmt.Errorf("generating invocation: %w", err)
	}

	res, err := client.Execute(ctx, []invocation.Invocation{inv}, s.conn)
	if err != nil {
		return nil, fmt.Errorf("sending invocation: %w", err)
	}

	reader, err := receipt.NewReceiptReaderFromTypes[blob.AcceptOk, fdm.FailureModel](blob.AcceptOkType(), fdm.FailureType(), types.Converters...)
	if err != nil {
		return nil, fmt.Errorf("generating receipt reader: %w", err)
	}

	rcptLink, ok := res.Get(inv.Link())
	if !ok {
		return nil, ErrNoReceipt
	}

	rcpt, err := reader.Read(rcptLink, res.Blocks())
	if err != nil {
		return nil, fmt.Errorf("reading receipt: %w", err)
	}

	acc, err := result.Unwrap(result.MapError(rcpt.Out(), failure.FromFailureModel))
	if err != nil {
		return nil, fmt.Errorf("received error from storage node: %w", err)
	}

	br, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(res.Blocks()))
	if err != nil {
		return nil, fmt.Errorf("setting up block reader: %w", err)
	}

	claim, err := delegation.NewDelegationView(acc.Site, br)
	if err != nil {
		return nil, fmt.Errorf("reading claim delegation: %w", err)
	}
	if len(claim.Capabilities()) != 1 || claim.Capabilities()[0].Can() != assert.LocationAbility {
		return nil, ErrIncorrectCapability
	}

	lc, err := assert.LocationCaveatsReader.Read(claim.Capabilities()[0].Nb())
	if err != nil {
		return nil, fmt.Errorf("decoding location commitment: %w", err)
	}
	result := &BlobAcceptResult{
		LocationCommitment: lc,
	}
	if acc.PDP != nil {
		pdpAccept, err := delegation.NewDelegationView(*acc.PDP, br)
		if err != nil {
			return nil, fmt.Errorf("reading piece accept invocation: %w", err)
		}
		if len(pdpAccept.Capabilities()) != 1 || pdpAccept.Capabilities()[0].Can() != pdp.AcceptAbility {
			return nil, ErrIncorrectCapability
		}
		ac, err := pdp.AcceptCaveatsReader.Read(pdpAccept.Capabilities()[0].Nb())
		if err != nil {
			return nil, fmt.Errorf("decoding location commitment: %w", err)
		}
		result.PDPAccept = &ac
	}
	return result, nil
}

func (s *Client) PDPInfo(ctx context.Context, pieceLink piece.PieceLink) (pdp.InfoOk, error) {

	inv, err := pdp.Info.Invoke(
		s.cfg.ID,
		s.cfg.StorageNodeID,
		s.cfg.StorageNodeID.DID().String(),
		pdp.InfoCaveats{
			Piece: pieceLink,
		},
		delegation.WithProof(s.cfg.StorageProof),
	)

	if err != nil {
		return pdp.InfoOk{}, fmt.Errorf("generating invocation: %w", err)
	}

	res, err := client.Execute(ctx, []invocation.Invocation{inv}, s.conn)
	if err != nil {
		return pdp.InfoOk{}, fmt.Errorf("sending invocation: %w", err)
	}

	reader, err := receipt.NewReceiptReaderFromTypes[pdp.InfoOk, fdm.FailureModel](pdp.InfoOkType(), fdm.FailureType(), types.Converters...)
	if err != nil {
		return pdp.InfoOk{}, fmt.Errorf("generating receipt reader: %w", err)
	}

	rcptLink, ok := res.Get(inv.Link())
	if !ok {
		return pdp.InfoOk{}, ErrNoReceipt
	}

	rcpt, err := reader.Read(rcptLink, res.Blocks())
	if err != nil {
		return pdp.InfoOk{}, fmt.Errorf("reading receipt: %w", err)
	}
	return result.Unwrap(result.MapError(rcpt.Out(), failure.FromFailureModel))
}

func NewClient(cfg Config) (*Client, error) {
	ch := uhttp.NewChannel(&cfg.StorageNodeURL)
	conn, err := client.NewConnection(cfg.StorageNodeID, ch)
	if err != nil {
		return nil, fmt.Errorf("setting up connection: %w", err)
	}
	return &Client{cfg, conn}, nil
}
