package pieces

import (
	"context"
	"errors"
	"fmt"

	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var log = logging.Logger("resolver")

// Resolver resolves externally supplied piece identifiers to the multihashes
// used within the blobstore.
type Resolver interface {
	Resolve(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error)
	ResolveFromCommp(ctx context.Context, cid cid.Cid) (multihash.Multihash, bool, error)
}

type StoreResolver struct {
	db    *gorm.DB
	store blobstore.Blobstore
	// TODO add an Arc or LRU cache
}

func (r *StoreResolver) ResolveFromCommp(ctx context.Context, piece cid.Cid) (multihash.Multihash, bool, error) {
	switch piece.Prefix().MhType {
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree): // we are resolving a CommP to the multihash it was uploaded with, which could be the commP, or a different mh
		var record models.PDPPieceMHToCommp
		log.Errorw("to commp", "cid", piece.String())
		if err := r.db.WithContext(ctx).Where("commp = ?", piece.String()).First(&record).Error; err != nil {
			// if the commp doesn't exist in the mapping, then the pice may have been uploaded as a commp and never
			// created a mapping, so query the store directly for it.
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if _, err := r.store.Get(ctx, piece.Hash()); err != nil {
					// the piece wasn't in the map, and it's not in the store, it doesn't exist in piri.
					if errors.Is(err, store.ErrNotFound) {
						return nil, false, nil
					}
					return nil, false, fmt.Errorf("failed to read blobstore: %w", err)
				}
				// the piece exists in the store, return it
				return piece.Hash(), true, nil
			}
			return nil, false, fmt.Errorf("failed to read database: %w", err)
		}

		// the piece exists in the map, decode it and return
		read, mh, err := multihash.MHFromBytes(record.Mhash)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read multihash: %w", err)
		}
		if read != len(record.Mhash) {
			return nil, false, fmt.Errorf("multihash read mismatch expected %d got %d", len(record.Mhash), read)
		}
		return mh, true, nil
	default: // we resolve the mh to the mh it was uploaded with iff it exists in the store
		if _, err := r.store.Get(ctx, piece.Hash()); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("failed to check blobstore: %w", err)
		}

		return piece.Hash(), true, nil
	}
}

func NewStoreResolver(db *gorm.DB, store blobstore.Blobstore) *StoreResolver {
	return &StoreResolver{
		db:    db,
		store: store,
	}
}

var _ Resolver = (*StoreResolver)(nil)

func (r *StoreResolver) Resolve(ctx context.Context, piece multihash.Multihash) (multihash.Multihash, bool, error) {
	log.Errorw("resolving piece", "hex", piece.HexString(), "base58", piece.B58String())
	dmh, err := multihash.Decode(piece)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decode multihash: %w", err)
	}
	switch dmh.Code {
	case uint64(multicodec.Fr32Sha256Trunc254Padbintree): // we are resolving a CommP to the multihash it was uploaded with, which could be the commP, or a different mh
		var record models.PDPPieceMHToCommp
		tmp := cid.NewCidV1(dmh.Code, dmh.Digest)
		digest, _, err := commcid.PieceCidV2ToDataCommitment(tmp)
		if err != nil {
			return nil, false, err
		}
		_, commpCID, err := cid.CidFromBytes(digest)
		if err != nil {
			return nil, false, err
		}
		log.Errorw("to commp", "cid", commpCID.String())
		if err := r.db.WithContext(ctx).Where("commp = ?", commpCID.String()).First(&record).Error; err != nil {
			// if the commp doesn't exist in the mapping, then the pice may have been uploaded as a commp and never
			// created a mapping, so query the store directly for it.
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if _, err := r.store.Get(ctx, piece); err != nil {
					// the piece wasn't in the map, and it's not in the store, it doesn't exist in piri.
					if errors.Is(err, store.ErrNotFound) {
						return nil, false, nil
					}
					return nil, false, fmt.Errorf("failed to read blobstore: %w", err)
				}
				// the piece exists in the store, return it
				return piece, true, nil
			}
			return nil, false, fmt.Errorf("failed to read database: %w", err)
		}

		// the piece exists in the map, decode it and return
		read, mh, err := multihash.MHFromBytes(record.Mhash)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read multihash: %w", err)
		}
		if read != len(record.Mhash) {
			return nil, false, fmt.Errorf("multihash read mismatch expected %d got %d", len(record.Mhash), read)
		}
		return mh, true, nil
	default: // we resolve the mh to the mh it was uploaded with iff it exists in the store
		if _, err := r.store.Get(ctx, piece); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("failed to check blobstore: %w", err)
		}

		return piece, true, nil
	}
}
