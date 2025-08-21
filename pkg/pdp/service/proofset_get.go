package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/types"
)

type ProofSet struct {
	ID                 int64
	Roots              []RootEntry
	NextChallengeEpoch int64
}

type RootEntry struct {
	RootID        uint64 `json:"rootId"`
	RootCID       string `json:"rootCid"`
	SubrootCID    string `json:"subrootCid"`
	SubrootOffset int64  `json:"subrootOffset"`
}

func (p *PDPService) GetProofSet(ctx context.Context, id uint64) (res *types.ProofSet, retErr error) {
	log.Infow("getting proof set", "id", id)
	defer func() {
		if retErr != nil {
			log.Errorw("failed to get proof set", "id", id, "err", retErr)
		} else {
			log.Infow("got proof set", "id", id, "response", res)
		}
	}()
	// Retrieve the proof set record.
	var proofSet models.PDPProofSet
	if err := p.db.WithContext(ctx).First(&proofSet, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, types.NewErrorf(types.KindNotFound, "proof set %d not found", id)
		}
		return nil, fmt.Errorf("failed to retrieve proof set: %w", err)
	}

	if proofSet.Service != p.name {
		return nil, types.NewError(types.KindUnauthorized, "not authorized")
	}

	// Retrieve the roots associated with the proof set.
	var roots []models.PDPProofsetRoot
	if err := p.db.WithContext(ctx).
		Where("proofset_id = ?", id).
		Order("root_id, subroot_offset").
		Find(&roots).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve proof set roots: %w", err)
	}

	// Step 5: Build the response.
	response := &types.ProofSet{
		ID:          uint64(proofSet.ID),
		Initialized: proofSet.InitReady,
	}
	for _, r := range roots {
		rootCid, err := cid.Decode(r.Root)
		if err != nil {
			return nil, fmt.Errorf("failed to decode root cid %s for proof set %d: %w", r.Root, proofSet.ID, err)
		}
		subrootCid, err := cid.Decode(r.Subroot)
		if err != nil {
			return nil, fmt.Errorf("failed to decode subroot cid %s for proof set %d: %w", r.Subroot, proofSet.ID, err)
		}
		response.Roots = append(response.Roots, types.RootEntry{
			RootID:        uint64(r.RootID),
			RootCID:       rootCid,
			SubrootCID:    subrootCid,
			SubrootOffset: r.SubrootOffset,
		})
	}
	if proofSet.ProveAtEpoch != nil {
		response.NextChallengeEpoch = *proofSet.ProveAtEpoch
	}
	if proofSet.PrevChallengeRequestEpoch != nil {
		response.PreviousChallengeEpoch = *proofSet.PrevChallengeRequestEpoch
	}
	if proofSet.ProvingPeriod != nil {
		response.ProvingPeriod = *proofSet.ProvingPeriod
	}
	if proofSet.ChallengeWindow != nil {
		response.ChallengeWindow = *proofSet.ChallengeWindow
	}

	return response, nil
}
