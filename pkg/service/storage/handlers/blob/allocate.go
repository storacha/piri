package blob

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/presets"

	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

var log = logging.Logger("storage/handlers/blob")

type AllocateService interface {
	PDP() pdp.PDP
	Blobs() blobs.Blobs
}

type AllocateRequest struct {
	Space did.DID
	Blob  captypes.Blob
	Cause ucan.Link
}

type AllocateResponse struct {
	Size    uint64
	Address *blob.Address
}

func Allocate(ctx context.Context, s AllocateService, req *AllocateRequest) (*AllocateResponse, error) {
	log := log.With("blob", digestutil.Format(req.Blob.Digest))
	log.Infof("%s space: %s", blob.AllocateAbility, req.Space)

	// check if we already have an allocation for the blob in this space
	allocs, err := s.Blobs().Allocations().List(ctx, req.Blob.Digest)
	if err != nil {
		log.Errorw("getting allocations", "error", err)
		return nil, fmt.Errorf("getting allocations: %w", err)
	}

	allocated := false
	for _, a := range allocs {
		if a.Space == req.Space {
			allocated = true
			break
		}
	}

	received := false
	// check if we received the blob (only possible if we have an allocation)
	if len(allocs) > 0 {
		if s.PDP() != nil {
			has, err := s.PDP().API().Has(ctx, req.Blob.Digest)
			if err != nil {
				return nil, fmt.Errorf("getting blob: %w", err)
			}
			received = has
		} else {
			_, err = s.Blobs().Store().Get(ctx, req.Blob.Digest)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				log.Errorw("getting blob", "error", err)
				return nil, fmt.Errorf("getting blob: %w", err)
			}
			if err == nil {
				received = true
			}
		}
	}

	// the size reported in the receipt is the number of bytes allocated
	// in the space - if a previous allocation already exists, this has
	// already been done, so the allocation size is 0
	size := req.Blob.Size
	if allocated {
		log.Info("blob allocation already exists")
		size = 0
	}

	// nothing to do
	if allocated && received {
		log.Info("blob already received")
		return &AllocateResponse{
			Size: size,
			// NB: blob already received, therefor no address is needed for upload.
			Address: nil,
		}, nil
	}

	expiresIn := uint64(60 * 60 * 24) // 1 day
	expiresAt := uint64(time.Now().Unix()) + expiresIn

	var address *blob.Address
	// if not received yet, we need to generate a signed URL for the
	// upload, and include it in the receipt.
	if !received {
		var uploadURL url.URL
		headers := http.Header{}
		if s.PDP() == nil {
			// use standard blob upload
			uploadURL, headers, err = s.Blobs().Presigner().SignUploadURL(ctx, req.Blob.Digest, req.Blob.Size, expiresIn)
			if err != nil {
				log.Errorw("signing upload URL", "error", err)
				return nil, fmt.Errorf("signing upload URL: %w", err)
			}
		} else {
			dmh, err := multihash.Decode(req.Blob.Digest)
			if err != nil {
				log.Errorw("decoding digest", "error", err)
				return nil, fmt.Errorf("decoding digest: %w", err)
			}
			if _, ok := presets.HasherRegistry[dmh.Name]; !ok {
				return nil, fmt.Errorf("unsupported hash: %s", dmh.Name)
			}
			// use pdp service upload
			// TODO we need to provide backpressure to the upload service here
			// based on the number of roots we are currently allocating.
			resp, err := s.PDP().API().AllocatePiece(ctx, types.PieceAllocation{
				Piece: types.Piece{
					Name: dmh.Name,
					Hash: req.Blob.Digest,
					Size: int64(req.Blob.Size),
				},
			})
			if err != nil {
				log.Errorw("adding to pdp service", "error", err)
				return nil, fmt.Errorf("adding to pdp service: %w", err)
			}
			if resp.Allocated {
				uploadURL, err = s.PDP().API().WritePieceURL(resp.UploadID)
				if err != nil {
					log.Errorw("getting piece write URL", "error", err)
					return nil, fmt.Errorf("getting piece write URL: %w", err)
				}
			}
		}
		address = &blob.Address{
			URL:     uploadURL,
			Headers: headers,
			Expires: expiresAt,
		}
	}

	// even if a previous allocation was made in this space, we create
	// another for the new invocation.
	err = s.Blobs().Allocations().Put(ctx, allocation.Allocation{
		Space:   req.Space,
		Blob:    allocation.Blob(req.Blob),
		Expires: expiresAt,
		Cause:   req.Cause,
	})
	if err != nil {
		log.Errorw("putting allocation", "error", err)
		return nil, fmt.Errorf("putting allocation: %w", err)
	}

	a, err := s.Blobs().Allocations().List(ctx, req.Blob.Digest)
	if err != nil {
		log.Errorw("listing allocation after put", "error", err)
		return nil, fmt.Errorf("listing allocation after put: %w", err)
	}
	if len(a) < 1 {
		log.Error("failed to read allocation after write")
		return nil, fmt.Errorf("failed to read allocation after write")
	}
	log.Info("successfully read allocation after write")

	return &AllocateResponse{
		Size:    size,
		Address: address,
	}, nil

}
