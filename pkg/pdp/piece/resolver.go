package piece

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/store/blobstore"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store"
)

var log = logging.Logger("pdp/piece")

// Resolver resolves externally supplied piece identifiers to the multihashes
// used within the blobstore.
type Resolver interface {
	ResolvePiece(ctx context.Context, blob multihash.Multihash) (multihash.Multihash, bool, error)
}

type StoreResolver struct {
	db    *gorm.DB
	store blobstore.PDPStore
	// TODO add an Arc or LRU cache
}

type StoreResolverParams struct {
	fx.In

	DB    *gorm.DB `name:"engine_db"`
	Store blobstore.PDPStore
	// TODO pass in a cache interface
}

func NewStoreResolver(params StoreResolverParams) Resolver {
	return &StoreResolver{
		db:    params.DB,
		store: params.Store,
	}
}

var _ Resolver = (*StoreResolver)(nil)

func (r *StoreResolver) ResolvePiece(ctx context.Context, blob multihash.Multihash) (resolved multihash.Multihash, found bool, retErr error) {
	start := time.Now()
	defer func() {
		if found {
			log.Infow("resolved piece", "blob", blob.String(), "resolved", resolved.String(), "duration", time.Since(start))
		} else {
			if retErr == nil {
				log.Errorw("failed to resolve blob", "blob", blob.String(), "retErr", retErr, "duration", time.Since(start))
			} else {
				log.Infow("could not resolve blob", "blob", blob.String(), "duration", time.Since(start))
			}
		}
	}()
	dmh, err := multihash.Decode(blob)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode multihash: %w", err)
	}
	switch dmh.Code {
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree): // we are resolving a CommP to the multihash it was uploaded with, which could be the commP, or a different mh
		commpCID, err := MultihashToCommpV2CID(blob)
		if err != nil {
			return nil, false, fmt.Errorf("failed to convert blob to commp CID: %w", err)
		}
		var record models.PDPPieceMHToCommp
		if err := r.db.WithContext(ctx).Where("commp = ?", commpCID.String()).First(&record).Error; err != nil {
			// if the commp doesn't exist in the mapping, then the pice may have been uploaded as a commp and never
			// created a mapping, so query the store directly for it.
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if _, err := r.store.Get(ctx, blob); err != nil {
					// the blob wasn't in the map, and it's not in the store, it doesn't exist in piri.
					if errors.Is(err, store.ErrNotFound) {
						return nil, false, nil
					}
					return nil, false, fmt.Errorf("failed to read blobstore: %w", err)
				}
				// the blob exists in the store, return it
				return blob, true, nil
			}
			return nil, false, fmt.Errorf("failed to read database: %w", err)
		}

		// the blob exists in the map, decode it and return
		read, mh, err := multihash.MHFromBytes(record.Mhash)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read multihash: %w", err)
		}
		if read != len(record.Mhash) {
			return nil, false, fmt.Errorf("multihash read mismatch expected %d got %d", len(record.Mhash), read)
		}
		return mh, true, nil
	default: // we resolve the mh to the mh it was uploaded with iff it exists in the store
		if _, err := r.store.Get(ctx, blob); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("failed to check blobstore: %w", err)
		}

		return blob, true, nil
	}
}

func MultihashToCommpV2CID(mh multihash.Multihash) (cid.Cid, error) {
	return cid.NewCidV1(cid.Raw, mh), nil
	/*
		digest, payloadSize, err := commcid.PieceCidV2ToDataCommitment(cid.NewCidV1(cid.Raw, mh))
		if err != nil {
			return cid.Undef, err
		}
		return commcid.DataCommitmentToPieceCidv2(digest, payloadSize)

	*/
}
