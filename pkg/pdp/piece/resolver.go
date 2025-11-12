package piece

import (
	"context"
	"errors"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

// DefaultResolverCacheSize is the default size of the resolver cache
//
// NB(forrest): arrived on this value since:
// It stores 2 hash per entry, each are ~256 bytes
// 256 bytes * 2 * 100,000 = ~60MiB when cache is full
// 100,000 entries assuming each piece references a ~128MiB blob allows the
// cache to store ~1.5 TiB worth of referenced data
const DefaultResolverCacheSize = 100_000

var log = logging.Logger("pdp/piece")

type StoreResolver struct {
	db    *gorm.DB
	cache *lru.Cache[string, multihash.Multihash]
}

type StoreResolverParams struct {
	fx.In

	DB *gorm.DB `name:"engine_db"`
}

func NewStoreResolver(params StoreResolverParams) (types.PieceResolverAPI, error) {
	cache, err := lru.New[string, multihash.Multihash](DefaultResolverCacheSize)
	if err != nil {
		return nil, err
	}
	return &StoreResolver{
		db:    params.DB,
		cache: cache,
	}, nil
}

var _ types.PieceResolverAPI = (*StoreResolver)(nil)

// Resolve accepts any multihash and returns its counterpart if present, false otherwise.
// Resolve can be used as a `Has` method.
func (r *StoreResolver) Resolve(ctx context.Context, mh multihash.Multihash) (multihash.Multihash, bool, error) {
	dmh, err := multihash.Decode(mh)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode multihash: %w", err)
	}
	if dmh.Code == uint64(multicodec.Fr32Sha256Trunc254Padbintree) {
		return r.ResolveToBlob(ctx, mh)
	}

	return r.ResolveToPiece(ctx, mh)
}

// ResolveToBlob returns the blob multihash for the provided piece if it exists.
// It accepts a piece multihash with the expected encoding of fr32-sha256-trunc254-padbintree and returns
// the corresponding blob the piece was derived from.
// If the piece does not exist it returns false and no error.
func (r *StoreResolver) ResolveToBlob(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error) {
	if cached, hit := r.cache.Get(piece.String()); hit {
		return cached, true, nil
	}
	dmh, err := multihash.Decode(piece)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode multihash: %w", err)
	}
	if dmh.Code != uint64(multicodec.Fr32Sha256Trunc254Padbintree) {
		return nil, false, fmt.Errorf("cannot resolve piece with codec %s to blob", multicodec.Fr32Sha256Trunc254Padbintree.String())
	}

	var record models.PDPPieceMHToCommp
	if err := r.db.WithContext(ctx).
		Where("commp = ?", MultihashToCommpCID(piece).String()).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read database: %w", err)
	}
	r.cache.Add(piece.String(), record.Mhash)
	return record.Mhash, true, nil
}

// ResolveToPiece returns the piece multihash for the provided blob if it exists.
// It accepts a piece multihash with any codec EXCEPT fr32-sha256-trunc254-padbintree and returns
// the corresponding piece derived from the blob.
// If the piece does not exist it returns false and no error.
func (r *StoreResolver) ResolveToPiece(ctx context.Context, blob multihash.Multihash) (multihash.Multihash, bool, error) {
	if cached, hit := r.cache.Get(blob.String()); hit {
		return cached, true, nil
	}
	dmh, err := multihash.Decode(blob)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode multihash: %w", err)
	}
	if dmh.Code == uint64(multicodec.Fr32Sha256Trunc254Padbintree) {
		// NB(forrest): we could return the blob here, but don't since the intention of this method is to return a commp mh for a non-commp mh iff one has been created
		return nil, false, fmt.Errorf("cannot resolve blob with codec %s to commp", multicodec.Fr32Sha256Trunc254Padbintree.String())
	}

	var record models.PDPPieceMHToCommp
	err = r.db.WithContext(ctx).
		Where("mhash = ?", blob.String()).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// not found isn't an error
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read database: %w", err)
	}

	commpCID, err := cid.Decode(record.Commp)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode commp cid %s for blob %s: %w", record.Commp, blob.String(), err)
	}
	r.cache.Add(blob.String(), commpCID.Hash())
	return commpCID.Hash(), true, nil

}

func MultihashToCommpCID(mh multihash.Multihash) cid.Cid {
	return cid.NewCidV1(cid.Raw, mh)
}
