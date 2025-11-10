package piece

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store"
)

var log = logging.Logger("pdp/piece")

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

func NewStoreResolver(params StoreResolverParams) types.PieceResolverAPI {
	return &StoreResolver{
		db:    params.DB,
		store: params.Store,
	}
}

var _ types.PieceResolverAPI = (*StoreResolver)(nil)

// ResolvePiece method can be thought of as an enhanced Has method, but with some important nuances:
// 1. If the codec of `blob`
func (r *StoreResolver) ResolvePiece(ctx context.Context, blob multihash.Multihash) (resolved multihash.Multihash, found bool, retErr error) {
	dmh, err := multihash.Decode(blob)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode multihash: %w", err)
	}

	switch dmh.Code {
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree): // we are resolving a CommP to the multihash it was uploaded with, which could be the commP, or a different mh
		commpCID := MultihashToCommpCID(blob)
		var record models.PDPPieceMHToCommp
		if err := r.db.WithContext(ctx).Where("commp = ?", commpCID.String()).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, false, nil
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
	default:
		// we resolve the mh to the mh it was uploaded with iff it exists in the store, this includes the case of Sha2_256Trunc254Padded
		// since it will be stored
		if _, err := r.store.Get(ctx, blob); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("failed to check blobstore: %w", err)
		}

		return blob, true, nil
	}
}

func MultihashToCommpCID(mh multihash.Multihash) cid.Cid {
	return cid.NewCidV1(cid.Raw, mh)
}
