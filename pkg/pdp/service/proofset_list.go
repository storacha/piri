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

func (p *PDPService) ListProofSets(ctx context.Context) ([]types.ProofSet, error) {
	var proofSets []models.PDPProofSet
	if err := p.db.
		WithContext(ctx).
		Where("service = ?", p.name).
		Find(&proofSets).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, types.NewErrorf(types.KindNotFound, "no proof sets found")
		}
		return nil, fmt.Errorf("failed to retrieve proof sets: %w", err)
	}

	// Build the response for each proof set
	result := make([]types.ProofSet, 0, len(proofSets))
	for _, proofSet := range proofSets {
		// Retrieve the roots associated with the proof set
		var roots []models.PDPProofsetRoot
		if err := p.db.WithContext(ctx).
			Where("proofset_id = ?", proofSet.ID).
			Order("root_id, subroot_offset").
			Find(&roots).Error; err != nil {
			return nil, fmt.Errorf("failed to retrieve proof set roots for proof set %d: %w", proofSet.ID, err)
		}

		// Build the response
		response := types.ProofSet{
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

		result = append(result, response)
	}

	return result, nil
}
