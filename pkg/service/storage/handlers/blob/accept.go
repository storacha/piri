package blob

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipfs/go-cid"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	pdp_cap "github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/acceptancestore/acceptance"
)

type AcceptService interface {
	ID() principal.Signer
	PDP() pdp.PDP
	Blobs() blobs.Blobs
	Claims() claims.Claims
}

type AcceptRequest struct {
	Space did.DID
	Blob  types.Blob
	Put   blob.Promise
	// Cause is a link to the `blob/accept` invocation.
	Cause ipld.Link
}

type AcceptResponse struct {
	Claim delegation.Delegation
	// only present when using PDP
	PDP invocation.Invocation
}

func Accept(ctx context.Context, s AcceptService, req *AcceptRequest) (*AcceptResponse, error) {
	log := log.With("blob", digestutil.Format(req.Blob.Digest))
	log.Infof("%s %s", blob.AcceptAbility, req.Space)

	var (
		err          error
		loc          url.URL
		pdpAcceptInv invocation.Invocation
	)
	if s.PDP() == nil {
		_, err = s.Blobs().Store().Get(ctx, req.Blob.Digest)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, fmt.Errorf("blob not found: %w", err)
			}
			log.Errorw("getting blob", "error", err)
			return nil, fmt.Errorf("getting blob: %w", err)
		}

		loc, err = s.Blobs().Access().GetDownloadURL(req.Blob.Digest)
		if err != nil {
			log.Errorw("creating retrieval URL for blob", "error", err)
			return nil, fmt.Errorf("creating retrieval URL for blob: %w", err)
		}
	} else {
		// ensure the blob exists, else it cannot be accepted.
		found, err := s.PDP().API().Has(ctx, req.Blob.Digest)
		if err != nil {
			log.Errorw("finding piece for blob", "error", err)
			return nil, fmt.Errorf("finding piece for blob: %w", err)
		}
		if !found {
			log.Errorw("piece not found", "blob", req.Blob.Digest)
			return nil, fmt.Errorf("piece not found: %w", err)
		}
		// get a download url
		blobCID := cid.NewCidV1(cid.Raw, req.Blob.Digest)
		loc, err = s.PDP().API().ReadPieceURL(blobCID)
		if err != nil {
			log.Errorw("creating retrieval URL for blob", "error", err)
			return nil, fmt.Errorf("creating retrieval URL for blob: %w", err)
		}
		// submit the piece for aggregation
		if err := s.PDP().CommpCalculate().Enqueue(ctx, req.Blob.Digest); err != nil {
			log.Errorw("submitting piece for aggregation", "error", err)
			return nil, fmt.Errorf("submitting piece for aggregation: %w", err)
		}
		// generate the invocation that will complete when aggregation is complete and the piece is accepted
		pieceAccept, err := pdp_cap.Accept.Invoke(
			s.ID(),
			s.ID(),
			s.ID().DID().String(),
			pdp_cap.AcceptCaveats{
				Blob: req.Blob.Digest,
			}, delegation.WithNoExpiration())
		if err != nil {
			log.Error("creating piece accept invocation", "error", err)
			return nil, fmt.Errorf("creating piece accept invocation: %w", err)
		}
		pdpAcceptInv = pieceAccept
	}

	byteRange := assert.Range{Offset: 0, Length: &req.Blob.Size}
	claim, err := assert.Location.Delegate(
		s.ID(),
		req.Space,
		s.ID().DID().String(),
		assert.LocationCaveats{
			Space:    req.Space,
			Content:  types.FromHash(req.Blob.Digest),
			Location: []url.URL{loc},
			Range:    &byteRange,
		},
		delegation.WithNoExpiration(),
	)
	if err != nil {
		log.Errorw("creating location commitment", "error", err)
		return nil, fmt.Errorf("creating location commitment: %w", err)
	}

	acc := acceptance.Acceptance{
		Space: req.Space,
		Blob: acceptance.Blob{
			Digest: req.Blob.Digest,
			Size:   req.Blob.Size,
		},
		ExecutedAt: uint64(time.Now().Unix()),
		Cause:      req.Cause,
	}
	if pdpAcceptInv != nil {
		acc.PDPAccept = &acceptance.Promise{
			UcanAwait: acceptance.Await{
				Selector: ".out.ok",
				Link:     pdpAcceptInv.Link(),
			},
		}
	}
	err = s.Blobs().Acceptances().Put(ctx, acc)
	if err != nil {
		log.Errorw("putting acceptance for blob", "error", err)
		return nil, fmt.Errorf("putting acceptance for blob: %w", err)
	}

	err = s.Claims().Store().Put(ctx, claim)
	if err != nil {
		log.Errorw("putting location claim for blob", "error", err)
		return nil, fmt.Errorf("putting location claim for blob: %w", err)
	}

	err = s.Claims().Publisher().Publish(ctx, claim)
	if err != nil {
		log.Errorw("publishing location commitment", "error", err)
		return nil, fmt.Errorf("publishing location commitment: %w", err)
	}

	return &AcceptResponse{
		Claim: claim,
		PDP:   pdpAcceptInv,
	}, nil
}
