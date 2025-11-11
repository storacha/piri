package piece

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

var log = logging.Logger("pdp/piece")

type StoreResolver struct {
	db *gorm.DB
	// TODO add an Arc or LRU cache
}

type StoreResolverParams struct {
	fx.In

	DB *gorm.DB `name:"engine_db"`
	// TODO pass in a cache interface
}

func NewStoreResolver(params StoreResolverParams) types.PieceResolverAPI {
	return &StoreResolver{
		db: params.DB,
	}
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
	return record.Mhash, true, nil
}

// ResolveToPiece returns the piece multihash for the provided blob if it exists.
// It accepts a piece multihash with any codec EXCEPT fr32-sha256-trunc254-padbintree and returns
// the corresponding piece derived from the blob.
// If the piece does not exist it returns false and no error.
func (r *StoreResolver) ResolveToPiece(ctx context.Context, blob multihash.Multihash) (multihash.Multihash, bool, error) {
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
	return commpCID.Hash(), true, nil

}

func MultihashToCommpCID(mh multihash.Multihash) cid.Cid {
	return cid.NewCidV1(cid.Raw, mh)
}
